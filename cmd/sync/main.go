package main

import (
	"flag"
	"fmt"
	stdlog "log"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/go-kit/kit/log/level"
	"github.com/go-kit/log"
	"github.com/oklog/run"
	"github.com/philipgough/hashring-controller/pkg/controller"
	"github.com/philipgough/hashring-controller/pkg/signals"
	"github.com/philipgough/hashring-controller/pkg/sync"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

const (
	defaultListen = ":8080"

	resyncPeriod = time.Minute
)

var (
	masterURL  string
	kubeconfig string
	namespace  string

	listen string
)

func main() {
	flag.Parse()
	ctx := signals.SetupSignalHandler()

	cfg, err := clientcmd.BuildConfigFromFlags(masterURL, kubeconfig)
	if err != nil {
		stdlog.Fatalf("error building kubeconfig: %s", err.Error())
	}

	kubeClient, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		stdlog.Fatalf("error building kubernetes clientset: %s", err.Error())
	}

	l, err := net.Listen("tcp", defaultListen)
	if err != nil {
		stdlog.Fatalf("error listening on %s: %s", defaultListen, err.Error())
	}

	logger := log.NewLogfmtLogger(log.NewSyncWriter(os.Stderr))
	logger = log.WithPrefix(logger, "ts", log.DefaultTimestampUTC)
	logger = log.WithPrefix(logger, "caller", log.DefaultCaller)
	logger = log.With(logger, "component", "hashring-syncer")

	r := prometheus.NewRegistry()
	r.MustRegister(
		collectors.NewGoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
	)

	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.HandlerFor(r, promhttp.HandlerOpts{}))

	configMapInformer := kubeinformers.NewSharedInformerFactoryWithOptions(
		kubeClient,
		resyncPeriod,
		kubeinformers.WithNamespace(namespace),
		kubeinformers.WithTweakListOptions(func(options *metav1.ListOptions) {
			options.LabelSelector = labels.Set{controller.ConfigMapLabel: "true"}.String()
		}),
	)

	// todo expose config as flags
	controller := sync.NewController(
		ctx,
		configMapInformer.Core().V1().ConfigMaps(),
		kubeClient,
		namespace,
		logger,
		r,
		nil,
	)

	var g run.Group
	{
		g.Add(func() error {
			configMapInformer.Start(ctx.Done())
			return controller.Run(ctx, 1)
		},
			func(_ error) {

			},
		)
	}

	{
		g.Add(func() error {
			mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusOK)
			})
			if err := http.Serve(l, mux); err != nil && err != http.ErrServerClosed {
				return fmt.Errorf("server error: %w", err)
			}
			return nil
		},
			func(error) {
				l.Close()
			},
		)
	}

	if err := g.Run(); err != nil {
		stdlog.Fatalf("error running controller: %s", err.Error())
	}
	level.Info(logger).Log("msg", "controller stopped gracefully")
}

func init() {
	flag.StringVar(&kubeconfig, "kubeconfig", "", "Path to a kubeconfig. Only required if out-of-cluster.")
	flag.StringVar(&masterURL, "master", "", "The address of the Kubernetes API server. Overrides any value in kubeconfig. Only required if out-of-cluster.")
	flag.StringVar(&namespace, "namespace", metav1.NamespaceDefault, "The namespace to watch")
	flag.StringVar(&listen, "listen", defaultListen, "The address to listen on")
}

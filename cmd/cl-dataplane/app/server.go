package app

import (
	"fmt"
	"os"

	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/clusterlink-net/clusterlink/pkg/dataplane/api"
	"github.com/clusterlink-net/clusterlink/pkg/util"
)

const (
	// logLevel is the default log level.
	logLevel = "warn"

	// CAFile is the path to the certificate authority file.
	CAFile = "/etc/ssl/certs/clink_ca.pem"
	// CertificateFile is the path to the certificate file.
	CertificateFile = "/etc/ssl/certs/clink-dataplane.pem"
	// KeyFile is the path to the private-key file.
	KeyFile = "/etc/ssl/private/clink-dataplane.pem"
)

// Options contains everything necessary to create and run a dataplane.
type Options struct {
	// ControlplaneHost is the IP/hostname of the controlplane.
	ControlplaneHost string
	// Type is the dataplane type.
	Type string
	// LogFile is the path to file where logs will be written.
	LogFile string
	// LogLevel is the log level.
	LogLevel string
}

// AddFlags adds flags to fs and binds them to options.
func (o *Options) AddFlags(fs *pflag.FlagSet) {
	fs.StringVar(&o.ControlplaneHost, "controlplane-host", "",
		"The controlplane IP/hostname.")
	fs.StringVar(&o.Type, "type", "clink",
		"The type of the dataplane. One of clink, envoy.")
	fs.StringVar(&o.LogFile, "log-file", "",
		"Path to a file where logs will be written. If not specified, logs will be printed to stderr.")
	fs.StringVar(&o.LogLevel, "log-level", logLevel,
		"The log level. One of fatal, error, warn, info, debug.")
}

// RequiredFlags are the names of flags that must be explicitly specified.
func (o *Options) RequiredFlags() []string {
	return []string{"controlplane-host"}
}

// Run the dataplane.
func (o *Options) Run() error {
	// set log file
	if o.LogFile != "" {
		f, err := os.OpenFile(o.LogFile, os.O_APPEND|os.O_CREATE|os.O_RDWR, 0666)
		if err != nil {
			return fmt.Errorf("unable to open log file: %v", err)
		}

		defer func() {
			if err := f.Close(); err != nil {
				log.Errorf("Cannot close log file: %v", err)
			}
		}()

		log.SetOutput(f)
	}

	// set log level
	logLevel, err := log.ParseLevel(o.LogLevel)
	if err != nil {
		return fmt.Errorf("unable to set log level: %v", err)
	}
	log.SetLevel(logLevel)

	// parse TLS files
	parsedCertData, err := util.ParseTLSFiles(CAFile, CertificateFile, KeyFile)
	if err != nil {
		return err
	}

	dnsNames := parsedCertData.DNSNames()
	if len(dnsNames) != 1 {
		return fmt.Errorf("expected peer certificate to contain a single DNS name, but got %d", len(dnsNames))
	}

	peerName, err := api.StripServerPrefix(dnsNames[0])
	if err != nil {
		return err
	}

	// generate random dataplane ID
	dataplaneID := uuid.New().String()
	log.Infof("Dataplane ID: %s.", dataplaneID)

	return o.runEnvoy(peerName, dataplaneID)
}

// NewCLDataplaneCommand creates a *cobra.Command object with default parameters.
func NewCLDataplaneCommand() *cobra.Command {
	opts := &Options{}

	cmd := &cobra.Command{
		Use:          "cl-dataplane",
		Long:         `cl-dataplane: dataplane agent for allowing network connectivity of remote clients and services`,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return opts.Run()
		},
	}

	opts.AddFlags(cmd.Flags())

	for _, flag := range opts.RequiredFlags() {
		if err := cmd.MarkFlagRequired(flag); err != nil {
			fmt.Printf("Error marking required flag '%s': %v\n", flag, err)
			os.Exit(1)
		}
	}

	return cmd
}

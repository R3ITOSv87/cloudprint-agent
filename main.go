package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/cloudprint/cloudprint-agent/api"
	"github.com/cloudprint/cloudprint-agent/config"
	"github.com/cloudprint/cloudprint-agent/discovery"
	printpkg "github.com/cloudprint/cloudprint-agent/print"
	"github.com/cloudprint/cloudprint-agent/service"
	"github.com/spf13/cobra"
)

var (
	cfgPath string
	apiURL  string
	version = service.Version
)

func main() {
	root := &cobra.Command{
		Use:   "cloudprint-agent",
		Short: "CloudPrint local printer agent",
		Long:  "Connects local network printers to the CloudPrint Mail Gateway cloud service.",
	}

	root.PersistentFlags().StringVar(&cfgPath, "config", "", "config file path (default: platform-specific)")
	root.PersistentFlags().StringVar(&apiURL, "api-url", "", "CloudPrint API URL")

	// register command
	registerCmd := &cobra.Command{
		Use:   "register",
		Short: "Register this agent with CloudPrint",
		RunE: func(cmd *cobra.Command, args []string) error {
			token, _ := cmd.Flags().GetString("token")
			name, _ := cmd.Flags().GetString("name")
			if token == "" {
				return fmt.Errorf("--token is required")
			}
			if name == "" {
				return fmt.Errorf("--name is required")
			}
			url := apiURL
			if url == "" {
				return fmt.Errorf("--api-url is required")
			}
			fmt.Printf("Registering agent \"%s\"...\n", name)
			resp, err := api.Register(url, token, name, version)
			if err != nil {
				return fmt.Errorf("registration failed: %w", err)
			}
			cfg := config.New(url, resp.AgentID, resp.APIKey, name)
			if err := config.Save(cfg, cfgPath); err != nil {
				return fmt.Errorf("save config: %w", err)
			}
			fmt.Printf("Registered! Agent ID: %s\n", resp.AgentID)
			fmt.Printf("Config saved to: %s\n", config.DefaultPath())
			fmt.Println("\nNext steps:")
			fmt.Println("  cloudprint-agent discover          — find printers in your network")
			fmt.Println("  cloudprint-agent run               — start the agent")
			fmt.Println("  cloudprint-agent install-service   — install as system service")
			return nil
		},
	}
	registerCmd.Flags().String("token", "", "Install token from CloudPrint dashboard (required)")
	registerCmd.Flags().String("name", "", "Agent name, e.g. 'Office Ground Floor' (required)")
	registerCmd.MarkFlagRequired("token")
	registerCmd.MarkFlagRequired("name")

	// run command
	runCmd := &cobra.Command{
		Use:   "run",
		Short: "Start the agent (foreground)",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(cfgPath)
			if err != nil {
				return fmt.Errorf("load config: %w", err)
			}
			ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
			defer cancel()
			poller := service.NewPoller(cfg)
			return poller.Run(ctx)
		},
	}

	// discover command
	discoverCmd := &cobra.Command{
		Use:   "discover",
		Short: "Discover printers in the local network",
		RunE: func(cmd *cobra.Command, args []string) error {
			subnet, _ := cmd.Flags().GetString("subnet")
			mdnsOnly, _ := cmd.Flags().GetBool("mdns")
			scanOnly, _ := cmd.Flags().GetBool("scan")

			var printers []discovery.Printer

			if !scanOnly {
				fmt.Println("Scanning via mDNS/Bonjour (5s)...")
				ctx := context.Background()
				mdnsPrinters, err := discovery.ScanMDNS(ctx, 5*time.Second)
				if err == nil {
					printers = append(printers, mdnsPrinters...)
				}
			}

			if !mdnsOnly {
				fmt.Printf("Scanning subnet %s for IPP (port 631)...\n", subnet)
				subnetPrinters := discovery.ScanSubnet(subnet, 500*time.Millisecond)
				printers = append(printers, subnetPrinters...)
			}

			// Also scan CUPS on Linux if neither flag restricts it
			if !mdnsOnly && !scanOnly {
				cupsPrinters := discovery.ScanCUPS()
				printers = append(printers, cupsPrinters...)
			}

			if len(printers) == 0 {
				fmt.Println("No printers found.")
				return nil
			}

			fmt.Printf("\nFound %d printer(s):\n\n", len(printers))
			for i, p := range printers {
				fmt.Printf("  [%d] %s\n", i+1, p.Name)
				fmt.Printf("      IPP URI: %s\n", p.IPPURI)
				if p.Model != "" {
					fmt.Printf("      Model:   %s\n", p.Model)
				}
				fmt.Println()
			}

			// Report to cloud if config exists
			cfg, err := config.Load(cfgPath)
			if err == nil && cfg.APIKey != "" {
				fmt.Println("Reporting discovered printers to CloudPrint...")
				client := api.New(cfg.APIURL, cfg.AgentID, cfg.APIKey)
				discovered := make([]api.DiscoveredPrinter, len(printers))
				for i, p := range printers {
					discovered[i] = api.DiscoveredPrinter{
						LocalName: p.Name,
						IPPURI:    p.IPPURI,
						Model:     p.Model,
						Reachable: true,
					}
				}
				if err := client.ReportPrinters(discovered); err != nil {
					fmt.Printf("Warning: could not report to cloud: %s\n", err)
				} else {
					fmt.Println("Printers reported. Enable them in the CloudPrint dashboard.")
				}
			}

			return nil
		},
	}
	discoverCmd.Flags().String("subnet", "", "Subnet to scan, e.g. 192.168.1.0/24 (auto-detected if empty)")
	discoverCmd.Flags().Bool("mdns", false, "mDNS/Bonjour only")
	discoverCmd.Flags().Bool("scan", false, "Subnet scan only")

	// status command
	statusCmd := &cobra.Command{
		Use:   "status",
		Short: "Show agent status and configured printers",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(cfgPath)
			if err != nil {
				return fmt.Errorf("not registered (config not found): %w", err)
			}
			fmt.Printf("Agent:    %s\n", cfg.AgentName)
			fmt.Printf("ID:       %s\n", cfg.AgentID)
			fmt.Printf("API URL:  %s\n", cfg.APIURL)
			fmt.Printf("Poll:     every %ds\n", cfg.PollInterval)
			fmt.Printf("Printers: %d configured\n\n", len(cfg.Printers))
			for _, p := range cfg.Printers {
				_, reachable := printpkg.ProbeIPP(p.IPPURI)
				status := "offline"
				if reachable {
					status = "online"
				}
				fmt.Printf("  %-30s  %s  %s\n", p.LocalName, p.IPPURI, status)
			}
			return nil
		},
	}

	// install-service command
	installSvcCmd := &cobra.Command{
		Use:   "install-service",
		Short: "Install cloudprint-agent as a system service",
		RunE: func(cmd *cobra.Command, args []string) error {
			return service.InstallService()
		},
	}

	// uninstall-service command
	uninstallSvcCmd := &cobra.Command{
		Use:   "uninstall-service",
		Short: "Uninstall the system service",
		RunE: func(cmd *cobra.Command, args []string) error {
			return service.UninstallService()
		},
	}

	// start-service command
	startSvcCmd := &cobra.Command{
		Use:   "start-service",
		Short: "Start the system service",
		RunE:  func(cmd *cobra.Command, args []string) error { return service.StartService() },
	}

	// stop-service command
	stopSvcCmd := &cobra.Command{
		Use:   "stop-service",
		Short: "Stop the system service",
		RunE:  func(cmd *cobra.Command, args []string) error { return service.StopService() },
	}

	root.AddCommand(
		registerCmd,
		runCmd,
		discoverCmd,
		statusCmd,
		installSvcCmd,
		uninstallSvcCmd,
		startSvcCmd,
		stopSvcCmd,
	)

	if err := root.Execute(); err != nil {
		os.Exit(1)
	}
}

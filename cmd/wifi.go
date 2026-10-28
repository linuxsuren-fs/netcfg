package cmd

import (
	"fmt"
	"strings"
	"syscall"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/LinuxSuRen/netcfg/internal/netconf"
)

var wifiListCmd = &cobra.Command{
	Use:   "list",
	Short: "Scan for visible Wi-Fi networks",
	RunE: func(cmd *cobra.Command, args []string) error {
		nets, err := netconf.NewManager().ScanWifi()
		if err != nil {
			return exitErr(cmd, err)
		}
		if outputJSON {
			printJSON(cmd.OutOrStdout(), nets)
			return nil
		}
		w := tabwriter.NewWriter(cmd.OutOrStdout(), 2, 4, 2, ' ', 0)
		fmt.Fprintln(w, "SSID\tSIGNAL\tSECURITY\tCHANNEL\tCURRENT")
		for _, net := range nets {
			current := ""
			if net.Current {
				current = "*"
			}
			fmt.Fprintf(w, "%s\t%d%%\t%s\t%s\t%s\n", net.SSID, net.Signal, net.Security, net.Channel, current)
		}
		if err := w.Flush(); err == nil && len(nets) == 0 {
			fmt.Fprintln(cmd.ErrOrStderr(), "no networks found; scanning may need administrator privileges on this system")
		}
		return nil
	},
}

var (
	wifiConnectIface    string
	wifiConnectPassword string
	wifiConnectDry      bool
)

var wifiConnectCmd = &cobra.Command{
	Use:   "connect <ssid>",
	Short: "Join a Wi-Fi network",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ssid := args[0]
		password := wifiConnectPassword
		if password == "" {
			raw, err := readPasswordPrompt(cmd)
			if err != nil {
				return exitErr(cmd, err)
			}
			password = raw
		}
		mgr := newManager(wifiConnectDry)
		if err := mgr.ConnectWifi(wifiConnectIface, ssid, password); err != nil {
			return exitErr(cmd, err)
		}
		if wifiConnectDry {
			reportDry(mgr)
			return nil
		}
		fmt.Fprintf(cmd.OutOrStdout(), "requested to join %s\n", ssid)
		return nil
	},
}

var (
	wifiAPIface    string
	wifiAPPassword string
	wifiAPChannel  int
	wifiAPBand     string
	wifiAPGateway  string
	wifiAPName     string
	wifiAPDry      bool
)

func readPasswordPrompt(cmd *cobra.Command) (string, error) {
	fmt.Fprint(cmd.ErrOrStderr(), "password (input hidden, empty for open network): ")
	raw, err := term.ReadPassword(int(syscall.Stdin))
	fmt.Fprintln(cmd.ErrOrStderr())
	if err != nil {
		return "", fmt.Errorf("read password: %v", err)
	}
	return strings.TrimSpace(string(raw)), nil
}

var wifiAPStartCmd = &cobra.Command{
	Use:   "start <ssid>",
	Short: "Start a Wi-Fi hotspot (AP mode)",
	Long: `Start a Wi-Fi hotspot (AP mode) sharing this machine's connectivity.

Creates a NetworkManager connection (default name netcfg-ap) in AP mode
with WPA2 when a password is given, or an open network without one.
Clients get addresses from the built-in shared DHCP (default gateway
10.42.0.1 on Linux).`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ssid := args[0]
		password := wifiAPPassword
		if password == "" && !cmd.Flags().Changed("password") {
			raw, err := readPasswordPrompt(cmd)
			if err != nil {
				return exitErr(cmd, err)
			}
			password = raw
		}
		opts := netconf.WifiAPOptions{
			Channel:  wifiAPChannel,
			Band:     wifiAPBand,
			Gateway:  wifiAPGateway,
			ConnName: wifiAPName,
		}
		mgr := newManager(wifiAPDry)
		if err := mgr.StartWifiAP(wifiAPIface, ssid, password, opts); err != nil {
			return exitErr(cmd, err)
		}
		if wifiAPDry {
			reportDry(mgr)
			return nil
		}
		fmt.Fprintf(cmd.OutOrStdout(), "hotspot %s started (stop with: netcfg wifi ap stop)\n", ssid)
		return nil
	},
}

var wifiAPStopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop the Wi-Fi hotspot started via `wifi ap start`",
	RunE: func(cmd *cobra.Command, args []string) error {
		opts := netconf.WifiAPOptions{ConnName: wifiAPName}
		mgr := newManager(wifiAPDry)
		if err := mgr.StopWifiAP(opts); err != nil {
			return exitErr(cmd, err)
		}
		if wifiAPDry {
			reportDry(mgr)
			return nil
		}
		fmt.Fprintln(cmd.OutOrStdout(), "hotspot stopped")
		return nil
	},
}

var (
	psIface     string
	psOn        bool
	psOff       bool
	psNoPersist bool
	psDry       bool
)

var wifiPowerSaveCmd = &cobra.Command{
	Use:   "power-save",
	Short: "Show or toggle Wi-Fi interface power save",
	Long: `Show or toggle Wi-Fi interface power save.

Without --on/--off prints the current state. Disabling power save is the
first thing to try when Wi-Fi clients of a hotspot (phones, tablets)
drop off intermittently. By default the setting is also persisted into
the interface's active connection profile so it survives reboots
(NetworkManager on Linux); --no-persist changes the runtime only.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		mgr := newManager(psDry)
		if !psOn && !psOff {
			enabled, err := mgr.WifiPowerSave(psIface)
			if err != nil {
				return exitErr(cmd, err)
			}
			if outputJSON {
				printJSON(cmd.OutOrStdout(), map[string]bool{"power_save": enabled})
				return nil
			}
			state := "off"
			if enabled {
				state = "on"
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Wi-Fi power save: %s\n", state)
			return nil
		}
		if psOn && psOff {
			return exitErr(cmd, fmt.Errorf("use either --on or --off, not both"))
		}
		if err := mgr.SetWifiPowerSave(psIface, psOn, !psNoPersist); err != nil {
			return exitErr(cmd, err)
		}
		if psDry {
			reportDry(mgr)
			return nil
		}
		fmt.Fprintln(cmd.OutOrStdout(), "Wi-Fi power save updated")
		return nil
	},
}

var wifiAPCmd = &cobra.Command{
	Use:   "ap",
	Short: "Start and stop a Wi-Fi hotspot (AP mode)",
}

var wifiCmd = &cobra.Command{
	Use:   "wifi",
	Short: "Scan and join Wi-Fi networks, or start a hotspot",
}

func init() {
	wifiCmd.AddCommand(wifiListCmd)
	f := wifiConnectCmd.Flags()
	f.StringVar(&wifiConnectIface, "iface", "", "interface to use (defaults to the platform's Wi-Fi interface)")
	f.StringVar(&wifiConnectPassword, "password", "", "pre-shared key (prompted when omitted)")
	f.BoolVar(&wifiConnectDry, "dry-run", false, "print the system commands instead of running them")
	wifiCmd.AddCommand(wifiConnectCmd)

	sf := wifiAPStartCmd.Flags()
	sf.StringVar(&wifiAPIface, "iface", "", "interface to use (defaults to the platform's Wi-Fi interface)")
	sf.StringVar(&wifiAPPassword, "password", "", "WPA2 pre-shared key, 8..63 chars (prompted when omitted; pass an empty value for an open network)")
	sf.IntVar(&wifiAPChannel, "channel", 0, "channel (0 = auto)")
	sf.StringVar(&wifiAPBand, "band", "bg", "frequency band: bg (2.4 GHz) or a (5 GHz)")
	sf.StringVar(&wifiAPGateway, "gateway", "", "hotspot gateway IPv4 (default 10.42.0.1 on Linux)")
	sf.StringVar(&wifiAPName, "name", "", "connection profile name (default netcfg-ap)")
	sf.BoolVar(&wifiAPDry, "dry-run", false, "print the system commands instead of running them")
	wifiAPCmd.AddCommand(wifiAPStartCmd)

	tf := wifiAPStopCmd.Flags()
	tf.StringVar(&wifiAPName, "name", "", "connection profile name (default netcfg-ap)")
	tf.BoolVar(&wifiAPDry, "dry-run", false, "print the system commands instead of running them")
	wifiAPCmd.AddCommand(wifiAPStopCmd)
	wifiCmd.AddCommand(wifiAPCmd)

	pf := wifiPowerSaveCmd.Flags()
	pf.StringVar(&psIface, "iface", "", "interface to use (defaults to the platform's Wi-Fi interface)")
	pf.BoolVar(&psOn, "on", false, "enable power save")
	pf.BoolVar(&psOff, "off", false, "disable power save (recommended for hotspots with intermittent client drops)")
	pf.BoolVar(&psNoPersist, "no-persist", false, "change the runtime only, do not write it into the connection profile")
	pf.BoolVar(&psDry, "dry-run", false, "print the system commands instead of running them")
	wifiCmd.AddCommand(wifiPowerSaveCmd)
	rootCmd.AddCommand(wifiCmd)
}

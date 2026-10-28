//go:build linux

package netconf

import (
	"strings"
	"testing"
)

func newPSManager() (*linuxManager, *DryRunner) {
	query := &fakeRunner{outputs: map[string]string{
		"nmcli -t -e no -f GENERAL.DEVICE,GENERAL.TYPE,GENERAL.HWADDR,GENERAL.MTU,GENERAL.STATE,GENERAL.CONNECTION,IP4.ADDRESS,IP4.GATEWAY,IP4.DNS device show": deviceShowOut,
		"nmcli -t -f NAME,DEVICE,ipv4.method connection show": "",
		"nmcli -t -e no -f ACTIVE,SSID device wifi":           "",
		"nmcli -t -f NAME connection show":                    "",
		// connectionOf(wlan0) walks device show for the specific device
		"nmcli -t -e no -f GENERAL.DEVICE,GENERAL.CONNECTION device show wlan0": "GENERAL.CONNECTION:unitree-ap",
	}}
	dry := &DryRunner{}
	return &linuxManager{query: query, mutate: dry, hasNM: true}, dry
}

func TestWifiPowerSaveRead(t *testing.T) {
	mgr, _ := newPSManager()
	mgr.query.(*fakeRunner).outputs["iw dev wlan0 get power_save"] = "Power save: on"
	on, err := mgr.WifiPowerSave("")
	if err != nil || !on {
		t.Fatalf("read on: enabled=%v err=%v", on, err)
	}
	mgr.query.(*fakeRunner).outputs["iw dev wlan0 get power_save"] = "Power save: off"
	on, err = mgr.WifiPowerSave("")
	if err != nil || on {
		t.Fatalf("read off: enabled=%v err=%v", on, err)
	}
}

func TestSetWifiPowerSavePersists(t *testing.T) {
	mgr, dry := newPSManager()
	if err := mgr.SetWifiPowerSave("wlan0", false, true); err != nil {
		t.Fatalf("set: %v", err)
	}
	want := []string{
		"iw dev wlan0 set power_save off",
		"nmcli connection modify unitree-ap 802-11-wireless.powersave 2",
	}
	for i := range want {
		if dry.Commands[i] != want[i] {
			t.Errorf("cmd[%d]:\n got %s\nwant %s", i, dry.Commands[i], want[i])
		}
	}
}

func TestSetWifiPowerSaveNoPersist(t *testing.T) {
	mgr, dry := newPSManager()
	if err := mgr.SetWifiPowerSave("wlan0", true, false); err != nil {
		t.Fatalf("set: %v", err)
	}
	if len(dry.Commands) != 1 || dry.Commands[0] != "iw dev wlan0 set power_save on" {
		t.Errorf("commands = %v", dry.Commands)
	}
}

func TestSetWifiPowerSavePersistWithoutConnection(t *testing.T) {
	mgr, _ := newPSManager()
	// no active connection on the device → specific error mentioning runtime applied
	mgr.query.(*fakeRunner).outputs["nmcli -t -e no -f GENERAL.DEVICE,GENERAL.CONNECTION device show wlan0"] = "GENERAL.CONNECTION:--"
	err := mgr.SetWifiPowerSave("wlan0", false, true)
	if err == nil {
		t.Fatal("expected error")
	}
	if CodeOf(err) != CodeNotFound {
		t.Errorf("code = %s want %s", CodeOf(err), CodeNotFound)
	}
	if !strings.Contains(err.Error(), "runtime") {
		t.Errorf("error should mention the runtime change: %v", err)
	}
}

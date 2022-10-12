package main

import (
	"encoding/xml"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"regexp"
	"strconv"

	"github.com/alecthomas/kong"
)

var testMode string

var CLI struct {
	WebListenAddress string `optional:"" name:"web.listen_address" help:"Address to listen on for web interface and telemetry." default::9202`
	WebTelemetryPath string `optional:"" name:"web.telemetry-path" help:"Path under which to expose metrics." default:/metrics`
	NvidiaSmiCommand string `optional:"" name:"nvidia-smi-command" help:"Path or command to be used for the nvidia-smi executable." default:nvidia-smi`
}

var (
	ErrorLogger *log.Logger
	InfoLogger  *log.Logger
)

type NvidiaSmiLog struct {
	DriverVersion string `xml:"driver_version"`
	CudaVersion   string `xml:"cuda_version"`
	AttachedGPUs  string `xml:"attached_gpus"`
	GPU           []struct {
		Id                       string `xml:"id,attr"`
		ProductName              string `xml:"product_name"`
		ProductBrand             string `xml:"product_brand"`
		DisplayMode              string `xml:"display_mode"`
		DisplayActive            string `xml:"display_active"`
		PersistenceMode          string `xml:"persistence_mode"`
		AccountingMode           string `xml:"accounting_mode"`
		AccountingModeBufferSize string `xml:"accounting_mode_buffer_size"`
		DriverModel              struct {
			CurrentDM string `xml:"current_dm"`
			PendingDM string `xml:"pending_dm"`
		} `xml:"driver_model"`
		Serial         string `xml:"serial"`
		UUID           string `xml:"uuid"`
		MinorNumber    string `xml:"minor_number"`
		VbiosVersion   string `xml:"vbios_version"`
		MultiGPUBoard  string `xml:"multigpu_board"`
		BoardId        string `xml:"board_id"`
		GPUPartNumber  string `xml:"gpu_part_number"`
		InfoRomVersion struct {
			ImgVersion string `xml:"img_version"`
			OemObject  string `xml:"oem_object"`
			EccObject  string `xml:"ecc_object"`
			PwrObject  string `xml:"pwr_object"`
		} `xml:"inforom_version"`
		GPUOperationMode struct {
			Current string `xml:"current_gom"`
			Pending string `xml:"pending_gom"`
		} `xml:"gpu_operation_mode"`
		GPUVirtualizationMode struct {
			VirtualizationMode string `xml:"virtualization_mode"`
			HostVGPUMode       string `xml:"host_vgpu_mode"`
		} `xml:"gpu_virtualization_mode"`
		IBMNPU struct {
			RelaxedOrderingMode string `xml:"relaxed_ordering_mode"`
		} `xml:"ibmnpu"`
		PCI struct {
			Bus         string `xml:"pci_bus"`
			Device      string `xml:"pci_device"`
			Domain      string `xml:"pci_domain"`
			DeviceId    string `xml:"pci_device_id"`
			BusId       string `xml:"pci_bus_id"`
			SubSystemId string `xml:"pci_sub_system_id"`
			GPULinkInfo struct {
				PCIeGen struct {
					Max     string `xml:"max_link_gen"`
					Current string `xml:"current_link_gen"`
				} `xml:"pcie_gen"`
				LinkWidth struct {
					Max     string `xml:"max_link_width"`
					Current string `xml:"current_link_width"`
				} `xml:"link_widths"`
			} `xml:"pci_gpu_link_info"`
			BridgeChip struct {
				Type string `xml:"bridge_chip_type"`
				Fw   string `xml:"bridge_chip_fw"`
			} `xml:"pci_bridge_chip"`
			ReplayCounter         string `xml:"replay_counter"`
			ReplayRolloverCounter string `xml:"replay_rollover_counter"`
			TxUtil                string `xml:"tx_util"`
			RxUtil                string `xml:"rx_util"`
		} `xml:"pci"`
		FanSpeed         string `xml:"fan_speed"`
		PerformanceState string `xml:"performance_state"`
		FbMemoryUsage struct {
			Total string `xml:"total"`
			Used  string `xml:"used"`
			Free  string `xml:"free"`
		} `xml:"fb_memory_usage"`
		Bar1MemoryUsage struct {
			Total string `xml:"total"`
			Used  string `xml:"used"`
			Free  string `xml:"free"`
		} `xml:"bar1_memory_usage"`
		ComputeMode string `xml:"compute_mode"`
		Utilization struct {
			GPUUtil     string `xml:"gpu_util"`
			MemoryUtil  string `xml:"memory_util"`
			EncoderUtil string `xml:"encoder_util"`
			DecoderUtil string `xml:"decoder_util"`
		} `xml:"utilization"`
		EncoderStats struct {
			SessionCount   string `xml:"session_count"`
			AverageFPS     string `xml:"average_fps"`
			AverageLatency string `xml:"average_latency"`
		} `xml:"encoder_stats"`
		FBCStats struct {
			SessionCount   string `xml:"session_count"`
			AverageFPS     string `xml:"average_fps"`
			AverageLatency string `xml:"average_latency"`
		} `xml:"fbc_stats"`
		Temperature struct {
			GPUTemp                string `xml:"gpu_temp"`
			GPUTempMaxThreshold    string `xml:"gpu_temp_max_threshold"`
			GPUTempSlowThreshold   string `xml:"gpu_temp_slow_threshold"`
			GPUTempMaxGpuThreshold string `xml:"gpu_temp_max_gpu_threshold"`
			MemoryTemp             string `xml:"memory_temp"`
			GPUTempMaxMemThreshold string `xml:"gpu_temp_max_mem_threshold"`
		} `xml:"temperature"`
		PowerReadings struct {
			PowerState         string `xml:"power_state"`
			PowerDraw          string `xml:"power_draw"`
			PowerLimit         string `xml:"power_limit"`
			DefaultPowerLimit  string `xml:"default_power_limit"`
			EnforcedPowerLimit string `xml:"enforced_power_limit"`
			MinPowerLimit      string `xml:"min_power_limit"`
			MaxPowerLimit      string `xml:"max_power_limit"`
		} `xml:"power_readings"`
		Clocks struct {
			GraphicsClock string `xml:"graphics_clock"`
			SmClock       string `xml:"sm_clock"`
			MemClock      string `xml:"mem_clock"`
			VideoClock    string `xml:"video_clock"`
		} `xml:"clocks"`
		MaxClocks struct {
			GraphicsClock string `xml:"graphics_clock"`
			SmClock       string `xml:"sm_clock"`
			MemClock      string `xml:"mem_clock"`
			VideoClock    string `xml:"video_clock"`
		} `xml:"max_clocks"`
		ClockPolicy struct {
			AutoBoost        string `xml:"auto_boost"`
			AutoBoostDefault string `xml:"auto_boost_default"`
		} `xml:"clock_policy"`
		Processes struct {
			ProcessInfo []struct {
				Pid         string `xml:"pid"`
				Type        string `xml:"type"`
				ProcessName string `xml:"process_name"`
				UsedMemory  string `xml:"used_memory"`
			} `xml:"process_info"`
		} `xml:"processes"`
	} `xml:"gpu"`
}

func formatVersion(key string, meta string, value string) string {
	r := regexp.MustCompile(`(?P<version>\d+\.\d+).*`)
	match := r.FindStringSubmatch(value)
	version := "0"
	if len(match) > 0 {
		version = match[1]
	}
	return formatValue(key, meta, version)
}

func formatValue(key string, meta string, value string) string {
	result := key
	if meta != "" {
		result += "{" + meta + "}"
	}
	return result + " " + value + "\n"
}

func filterUnit(s string) string {
	if s == "N/A" {
		return "0"
	}
	r := regexp.MustCompile(`(?P<value>[\d\.]+) (?P<power>[KMGT]?[i]?)(?P<unit>.*)`)
	match := r.FindStringSubmatch(s)
	if len(match) == 0 {
		return "0"
	}

	result := make(map[string]string)
	for i, name := range r.SubexpNames() {
		if i != 0 && name != "" {
			result[name] = match[i]
		}
	}

	power := result["power"]
	if value, err := strconv.ParseFloat(result["value"], 32); err == nil {
		switch power {
		case "K":
			value *= 1000
		case "M":
			value *= 1000 * 1000
		case "G":
			value *= 1000 * 1000 * 1000
		case "T":
			value *= 1000 * 1000 * 1000 * 1000
		case "Ki":
			value *= 1024
		case "Mi":
			value *= 1024 * 1024
		case "Gi":
			value *= 1024 * 1024 * 1024
		case "Ti":
			value *= 1024 * 1024 * 1024 * 1024
		}
		return fmt.Sprintf("%g", value)
	}
	return "0"
}

func filterNumber(value string) string {
	if value == "N/A" {
		return "0"
	}
	r := regexp.MustCompile("[^0-9.]")
	return r.ReplaceAllString(value, "")
}

func parseNvidiaSMIOutput(output []byte) NvidiaSmiLog {
	var xmlData NvidiaSmiLog
	xml.Unmarshal(output, &xmlData)
	return xmlData
}

func generateMetricsResponse(w http.ResponseWriter, xmlData NvidiaSmiLog) {
	for _, GPU := range xmlData.GPU {
		id_info := "id=\""+GPU.Id+"\",uuid=\""+GPU.UUID+"\",name=\""+GPU.ProductName+"\""
		io.WriteString(w, formatVersion("nvidia_smi_driver_version", id_info, xmlData.DriverVersion))
		io.WriteString(w, formatVersion("nvidia_smi_cuda_version", id_info, xmlData.CudaVersion))
		io.WriteString(w, formatValue("nvidia_smi_attached_gpus", id_info, xmlData.AttachedGPUs))
		io.WriteString(w, formatValue("nvidia_smi_pci_pcie_gen_max", id_info, GPU.PCI.GPULinkInfo.PCIeGen.Max))
		io.WriteString(w, formatValue("nvidia_smi_pci_pcie_gen_current", id_info, GPU.PCI.GPULinkInfo.PCIeGen.Current))
		io.WriteString(w, formatValue("nvidia_smi_pci_link_width_max_multiplicator", id_info, filterNumber(GPU.PCI.GPULinkInfo.LinkWidth.Max)))
		io.WriteString(w, formatValue("nvidia_smi_pci_link_width_current_multiplicator", id_info, filterNumber(GPU.PCI.GPULinkInfo.LinkWidth.Current)))
		io.WriteString(w, formatValue("nvidia_smi_pci_replay_counter", id_info, GPU.PCI.ReplayRolloverCounter))
		io.WriteString(w, formatValue("nvidia_smi_pci_replay_rollover_counter", id_info, GPU.PCI.ReplayRolloverCounter))
		io.WriteString(w, formatValue("nvidia_smi_pci_tx_util_bytes_per_second", id_info, filterUnit(GPU.PCI.TxUtil)))
		io.WriteString(w, formatValue("nvidia_smi_pci_rx_util_bytes_per_second", id_info, filterUnit(GPU.PCI.RxUtil)))
		io.WriteString(w, formatValue("nvidia_smi_fan_speed_percent", id_info, filterUnit(GPU.FanSpeed)))
		io.WriteString(w, formatValue("nvidia_smi_performance_state_int", id_info, filterNumber(GPU.PerformanceState)))
		io.WriteString(w, formatValue("nvidia_smi_fb_memory_usage_total_bytes", id_info, filterUnit(GPU.FbMemoryUsage.Total)))
		io.WriteString(w, formatValue("nvidia_smi_fb_memory_usage_used_bytes", id_info, filterUnit(GPU.FbMemoryUsage.Used)))
		io.WriteString(w, formatValue("nvidia_smi_fb_memory_usage_free_bytes", id_info, filterUnit(GPU.FbMemoryUsage.Free)))
		io.WriteString(w, formatValue("nvidia_smi_bar1_memory_usage_total_bytes", id_info, filterUnit(GPU.Bar1MemoryUsage.Total)))
		io.WriteString(w, formatValue("nvidia_smi_bar1_memory_usage_used_bytes", id_info, filterUnit(GPU.Bar1MemoryUsage.Used)))
		io.WriteString(w, formatValue("nvidia_smi_bar1_memory_usage_free_bytes", id_info, filterUnit(GPU.Bar1MemoryUsage.Free)))
		io.WriteString(w, formatValue("nvidia_smi_utilization_gpu_percent", id_info, filterUnit(GPU.Utilization.GPUUtil)))
		io.WriteString(w, formatValue("nvidia_smi_utilization_memory_percent", id_info, filterUnit(GPU.Utilization.MemoryUtil)))
		io.WriteString(w, formatValue("nvidia_smi_utilization_encoder_percent", id_info, filterUnit(GPU.Utilization.EncoderUtil)))
		io.WriteString(w, formatValue("nvidia_smi_utilization_decoder_percent", id_info, filterUnit(GPU.Utilization.DecoderUtil)))
		io.WriteString(w, formatValue("nvidia_smi_encoder_session_count", id_info, GPU.EncoderStats.SessionCount))
		io.WriteString(w, formatValue("nvidia_smi_encoder_average_fps", id_info, GPU.EncoderStats.AverageFPS))
		io.WriteString(w, formatValue("nvidia_smi_encoder_average_latency", id_info, GPU.EncoderStats.AverageLatency))
		io.WriteString(w, formatValue("nvidia_smi_fbc_session_count", id_info, GPU.FBCStats.SessionCount))
		io.WriteString(w, formatValue("nvidia_smi_fbc_average_fps", id_info, GPU.FBCStats.AverageFPS))
		io.WriteString(w, formatValue("nvidia_smi_fbc_average_latency", id_info, GPU.FBCStats.AverageLatency))
		io.WriteString(w, formatValue("nvidia_smi_gpu_temp_celsius", id_info, filterUnit(GPU.Temperature.GPUTemp)))
		io.WriteString(w, formatValue("nvidia_smi_gpu_temp_max_threshold_celsius", id_info, filterUnit(GPU.Temperature.GPUTempMaxThreshold)))
		io.WriteString(w, formatValue("nvidia_smi_gpu_temp_slow_threshold_celsius", id_info, filterUnit(GPU.Temperature.GPUTempSlowThreshold)))
		io.WriteString(w, formatValue("nvidia_smi_gpu_temp_max_gpu_threshold_celsius", id_info, filterUnit(GPU.Temperature.GPUTempMaxGpuThreshold)))
		io.WriteString(w, formatValue("nvidia_smi_memory_temp_celsius", id_info, filterUnit(GPU.Temperature.MemoryTemp)))
		io.WriteString(w, formatValue("nvidia_smi_gpu_temp_max_mem_threshold_celsius", id_info, filterUnit(GPU.Temperature.GPUTempMaxMemThreshold)))
		io.WriteString(w, formatValue("nvidia_smi_power_state_int", id_info, filterNumber(GPU.PowerReadings.PowerState)))
		io.WriteString(w, formatValue("nvidia_smi_power_draw_watts", id_info, filterUnit(GPU.PowerReadings.PowerDraw)))
		io.WriteString(w, formatValue("nvidia_smi_power_limit_watts", id_info, filterUnit(GPU.PowerReadings.PowerLimit)))
		io.WriteString(w, formatValue("nvidia_smi_default_power_limit_watts", id_info, filterUnit(GPU.PowerReadings.DefaultPowerLimit)))
		io.WriteString(w, formatValue("nvidia_smi_enforced_power_limit_watts", id_info, filterUnit(GPU.PowerReadings.EnforcedPowerLimit)))
		io.WriteString(w, formatValue("nvidia_smi_min_power_limit_watts", id_info, filterUnit(GPU.PowerReadings.MinPowerLimit)))
		io.WriteString(w, formatValue("nvidia_smi_max_power_limit_watts", id_info, filterUnit(GPU.PowerReadings.MaxPowerLimit)))
		io.WriteString(w, formatValue("nvidia_smi_clock_graphics_hertz", id_info, filterUnit(GPU.Clocks.GraphicsClock)))
		io.WriteString(w, formatValue("nvidia_smi_clock_graphics_max_hertz", id_info, filterUnit(GPU.MaxClocks.GraphicsClock)))
		io.WriteString(w, formatValue("nvidia_smi_clock_sm_hertz", id_info, filterUnit(GPU.Clocks.SmClock)))
		io.WriteString(w, formatValue("nvidia_smi_clock_sm_max_hertz", id_info, filterUnit(GPU.MaxClocks.SmClock)))
		io.WriteString(w, formatValue("nvidia_smi_clock_mem_hertz", id_info, filterUnit(GPU.Clocks.MemClock)))
		io.WriteString(w, formatValue("nvidia_smi_clock_mem_max_hertz", id_info, filterUnit(GPU.MaxClocks.MemClock)))
		io.WriteString(w, formatValue("nvidia_smi_clock_video_hertz", id_info, filterUnit(GPU.Clocks.VideoClock)))
		io.WriteString(w, formatValue("nvidia_smi_clock_video_max_hertz", id_info, filterUnit(GPU.MaxClocks.VideoClock)))
		io.WriteString(w, formatValue("nvidia_smi_clock_policy_auto_boost", id_info, filterUnit(GPU.ClockPolicy.AutoBoost)))
		io.WriteString(w, formatValue("nvidia_smi_clock_policy_auto_boost_default", id_info, filterUnit(GPU.ClockPolicy.AutoBoostDefault)))
		for _, Process := range GPU.Processes.ProcessInfo {
			io.WriteString(w, formatValue("nvidia_smi_process_used_memory_bytes", id_info+",process_pid=\""+Process.Pid+"\",process_type=\""+Process.Type+"\"", filterUnit(Process.UsedMemory)))
		}
	}
	return
}

func metrics(w http.ResponseWriter, r *http.Request) {
	InfoLogger.Print("Serving /metrics")

	var cmd *exec.Cmd
	cmd = exec.Command(CLI.NvidiaSmiCommand, "-q", "-x")

	// Execute nvidia smi command
	stdout, err := cmd.Output()
	if err != nil {
		ErrorLogger.Print("Failed to run nvidia-smi command " + err.Error())
		return
	}

	// Parse XML
	xmlData := parseNvidiaSMIOutput(stdout)

	// Write output
	generateMetricsResponse(w, xmlData)
}

func index(w http.ResponseWriter, r *http.Request) {
	InfoLogger.Print("Serving /index")
	html := `<!doctype html>
<html>
    <head>
        <meta charset="utf-8">
        <title>Nvidia SMI Exporter</title>
    </head>
    <body>
        <h1>Nvidia SMI Exporter</h1>
        <p><a href="/metrics">Metrics</a></p>
    </body>
</html>`
	io.WriteString(w, html)
}

func main() {
	log.SetFlags(log.Flags() | log.Lmicroseconds | log.Lshortfile | log.Lmsgprefix)
	InfoLogger = log.New(os.Stdout, "- INFO - ", log.Flags())
	ErrorLogger = log.New(os.Stdout, "- ERROR - ", log.Flags())

	kong.Parse(&CLI)

	testMode = os.Getenv("TEST_MODE")
	if testMode == "1" {
		InfoLogger.Print("Test mode is enabled")
	}
	InfoLogger.Print("Nvidia SMI exporter listening on " + CLI.WebListenAddress)
	http.HandleFunc("/", index)
	http.HandleFunc(CLI.WebTelemetryPath, metrics)
	http.ListenAndServe(CLI.WebListenAddress, nil)
}

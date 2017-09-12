package main

import (
    "io"
    "net/http"
    "encoding/xml"
    "os/exec"
    "log"
    "os"
    "fmt"
    "regexp"
)

var indexHtml = string(`
<!doctype html>
<html>
    <head>
        <meta charset="utf-8">
        <title>Nvidia SMI Exporter</title>
    </head>
    <body>
        <h1>Prometheus Nvidia SMI Exporter</h1>
        <p><a href="/metrics">Metrics</a></p>
    </body>
</html>
`)

type NvidiaSmiLog struct {
    DriverVersion string `xml:"driver_version"`
    AttachedGPUs string `xml:"attached_gpus"`
    GPUs []struct {
        ProductName string `xml:"product_name"`
        ProductBrand string `xml:"product_brand"`
        FanSpeed string `xml:"fan_speed"`
        PCI struct {
            PCIBus string `xml:"pci_bus"`
        } `xml:"pci"`
        FbMemoryUsage struct {
            Total string `xml:"total"`
            Used string `xml:"used"`
            Free string `xml:"free"`
        } `xml:"fb_memory_usage"`
        Utilization struct {
            GPUUtil string `xml:"gpu_util"`
            MemoryUtil string `xml:"memory_util"`
        } `xml:"utilization"`
        Temperature struct {
            GPUTemp string `xml:"gpu_temp"`
            GPUTempMaxThreshold string `xml:"gpu_temp_max_threshold"`
            GPUTempSlowThreshold string `xml:"gpu_temp_slow_threshold"`
        } `xml:"temperature"`
        PowerReadings struct {
            PowerDraw string `xml:"power_draw"`
            PowerLimit string `xml:"power_limit"`
        } `xml:"power_readings"`
        Clocks struct {
            GraphicsClock string `xml:"graphics_clock"`
            SmClock string `xml:"sm_clock"`
            MemClock string `xml:"mem_clock"`
            VideoClock string `xml:"video_clock"`
        } `xml:"clocks"`
        MaxClocks struct {
            GraphicsClock string `xml:"graphics_clock"`
            SmClock string `xml:"sm_clock"`
            MemClock string `xml:"mem_clock"`
            VideoClock string `xml:"video_clock"`
        } `xml:"max_clocks"`
    } `xml:"gpu"`
}

func formatValue(key string, meta string, value string) string {
    result := key;
    if (meta != "") {
        result += "{" + meta + "}";
    }
    result += " "
    result += filterNumber(value)
    result += "\n"
    return result
}

func filterNumber(value string) string {
    r := regexp.MustCompile("[^0-9.]")
    return r.ReplaceAllString(value, "")
}

func metrics(w http.ResponseWriter, r *http.Request) {
    log.Print("Serving /metrics")

    var cmd *exec.Cmd
    if (os.Getenv("TEST_MODE") == "1") {
        log.Print("Test mode enabled")
        dir, err := os.Getwd()
        if err != nil {
            log.Fatal(err)
        }
        cmd = exec.Command("/bin/cat", dir + "/test.xml")
    } else {
        cmd = exec.Command("/usr/bin/nvidia-smi", "-q", "-x")
    }

    stdout, err := cmd.Output()
    if err != nil {
        println(err.Error())
        return
    }

    // Parse XML
    var xmlData NvidiaSmiLog
    xml.Unmarshal(stdout, &xmlData)
    fmt.Println(xmlData)

    // Output
    io.WriteString(w, formatValue("nvidiasmi_driver_version", "",xmlData.DriverVersion))
    io.WriteString(w, formatValue("nvidiasmi_attached_gpus", "", filterNumber(xmlData.AttachedGPUs)))
    for _, GPU := range xmlData.GPUs {
        io.WriteString(w, formatValue("nvidiasmi_fan_speed", "gpu=\"" + GPU.PCI.PCIBus + "\"", filterNumber(GPU.FanSpeed)))
        io.WriteString(w, formatValue("nvidiasmi_memory_usage_total", "gpu=\"" + GPU.PCI.PCIBus + "\"", filterNumber(GPU.FbMemoryUsage.Total)))
        io.WriteString(w, formatValue("nvidiasmi_memory_usage_used", "gpu=\"" + GPU.PCI.PCIBus + "\"", filterNumber(GPU.FbMemoryUsage.Used)))
        io.WriteString(w, formatValue("nvidiasmi_memory_usage_free", "gpu=\"" + GPU.PCI.PCIBus + "\"", filterNumber(GPU.FbMemoryUsage.Free)))
        io.WriteString(w, formatValue("nvidiasmi_utilization_gpu", "gpu=\"" + GPU.PCI.PCIBus + "\"", filterNumber(GPU.Utilization.GPUUtil)))
        io.WriteString(w, formatValue("nvidiasmi_utilization_memory", "gpu=\"" + GPU.PCI.PCIBus + "\"", filterNumber(GPU.Utilization.MemoryUtil)))
        io.WriteString(w, formatValue("nvidiasmi_temp_gpu", "gpu=\"" + GPU.PCI.PCIBus + "\"", filterNumber(GPU.Temperature.GPUTemp)))
        io.WriteString(w, formatValue("nvidiasmi_temp_gpu_max", "gpu=\"" + GPU.PCI.PCIBus + "\"", filterNumber(GPU.Temperature.GPUTempMaxThreshold)))
        io.WriteString(w, formatValue("nvidiasmi_temp_gpu_slow", "gpu=\"" + GPU.PCI.PCIBus + "\"", filterNumber(GPU.Temperature.GPUTempSlowThreshold)))
        io.WriteString(w, formatValue("nvidiasmi_power_draw", "gpu=\"" + GPU.PCI.PCIBus + "\"", filterNumber(GPU.PowerReadings.PowerDraw)))
        io.WriteString(w, formatValue("nvidiasmi_power_limit", "gpu=\"" + GPU.PCI.PCIBus + "\"", filterNumber(GPU.PowerReadings.PowerLimit)))
        io.WriteString(w, formatValue("nvidiasmi_clock_graphics", "gpu=\"" + GPU.PCI.PCIBus + "\"", filterNumber(GPU.Clocks.GraphicsClock)))
        io.WriteString(w, formatValue("nvidiasmi_clock_graphics_max", "gpu=\"" + GPU.PCI.PCIBus + "\"", filterNumber(GPU.MaxClocks.GraphicsClock)))
        io.WriteString(w, formatValue("nvidiasmi_clock_sm", "gpu=\"" + GPU.PCI.PCIBus + "\"", filterNumber(GPU.Clocks.SmClock)))
        io.WriteString(w, formatValue("nvidiasmi_clock_sm_max", "gpu=\"" + GPU.PCI.PCIBus + "\"", filterNumber(GPU.MaxClocks.SmClock)))
        io.WriteString(w, formatValue("nvidiasmi_clock_mem", "gpu=\"" + GPU.PCI.PCIBus + "\"", filterNumber(GPU.Clocks.MemClock)))
        io.WriteString(w, formatValue("nvidiasmi_clock_mem_max", "gpu=\"" + GPU.PCI.PCIBus + "\"", filterNumber(GPU.MaxClocks.MemClock)))
        io.WriteString(w, formatValue("nvidiasmi_clock_video", "gpu=\"" + GPU.PCI.PCIBus + "\"", filterNumber(GPU.Clocks.VideoClock)))
        io.WriteString(w, formatValue("nvidiasmi_clock_video_max", "gpu=\"" + GPU.PCI.PCIBus + "\"", filterNumber(GPU.MaxClocks.VideoClock)))
    }
}

func index(w http.ResponseWriter, r *http.Request) {
    log.Print("Serving /index")
    io.WriteString(w, indexHtml)
}

func main() {
    log.Print("Prometheus Nvidia SMI Exporter running")
    http.HandleFunc("/", index)
    http.HandleFunc("/metrics", metrics)
    http.ListenAndServe(":9500", nil)
}
package main

import (
	"os"
	"testing"
)

func TestParse(t *testing.T){
	dat, err := os.ReadFile("nvidia-smi.sample.xml")
	if err != nil {
		panic(err)
	}
	var xmlData NvidiaSmiLog
	xmlData = parseNvidiaSMIOutput(dat)

	if xmlData.DriverVersion != "440.95.01" {
		t.Errorf("got %q, wanted %q", xmlData.DriverVersion, "440.95.01")
	}
	memoryUtil := filterUnit(xmlData.GPU[0].Utilization.MemoryUtil)
	if memoryUtil != "1" {
		t.Errorf("got %q, wanted %q", memoryUtil, "1")
	}
	clockRate := filterUnit(xmlData.GPU[0].Clocks.GraphicsClock)
	if clockRate != "9.61e+08" {
		t.Errorf("got %q, wanted %q", clockRate, "9.61e+08")
	}
}

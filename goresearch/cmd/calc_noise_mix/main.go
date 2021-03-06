package main

import (
	"flag"
	"fmt"
	"log"
	"math"
	"math/rand"
	"os"
	"path/filepath"
	"strconv"

	"github.com/go-audio/audio"
	"github.com/go-audio/wav"
	"gonum.org/v1/gonum/floats"

	"github.com/tetsuzawa/research-tools/goresearch"
)

func main() {

	log.SetFlags(log.LstdFlags | log.Lshortfile)

	var (
		cleanFilepath string
		noiseFilepath string
		outputDir     string
		snrStart      float64
		snrEnd        float64
		snrDiv        int
	)

	flag.StringVar(&cleanFilepath, "clean", "/path/to/clean_file.wav", "designate clean file path")
	flag.StringVar(&noiseFilepath, "noise", "/path/to/noise_file.wav", "designate noise file path")
	flag.StringVar(&outputDir, "output", "/path/to/dir/", "designate ouput directory")
	flag.Float64Var(&snrStart, "start", -40, "designate start value of S/N Rate")
	flag.Float64Var(&snrEnd, "end", 40, "designate end value of S/N Rate")
	flag.IntVar(&snrDiv, "div", 19, "designate number of divisions")

	flag.Parse()

	if cleanFilepath == "/path/to/clean_file.wav" ||
		noiseFilepath == "/path/to/noise_file.wav" ||
		outputDir == "/path/to/dir/" {
		flag.Usage()
		os.Exit(1)
	}
	fmt.Println("clean file path:", cleanFilepath)
	fmt.Println("noise file path:", noiseFilepath)
	fmt.Println("ouput directory:", outputDir)
	fmt.Println("start value of S/N Rate:", snrStart)
	fmt.Println("end value of S/N Rate:", snrEnd)
	fmt.Println("number of divisions:", snrDiv)

	f1, err := os.Open(cleanFilepath)
	check(err)
	w1 := wav.NewDecoder(f1)

	f2, err := os.Open(noiseFilepath)
	check(err)
	w2 := wav.NewDecoder(f2)

	w1.ReadInfo()
	w2.ReadInfo()
	ch1 := int(w1.NumChans)
	ch2 := int(w2.NumChans)
	bitDepth1 := int(w1.BitDepth)
	bitDepth2 := int(w2.BitDepth)
	bps1 := bitDepth1 / 8
	bps2 := bitDepth2 / 8
	fs1 := int(w1.SampleRate)
	fs2 := int(w2.SampleRate)

	buf1, err := w1.FullPCMBuffer()
	check(err)
	buf2, err := w2.FullPCMBuffer()
	check(err)

	err = f1.Close()
	check(err)
	err = f2.Close()
	check(err)

	if ch1 != ch2 ||
		bitDepth1 != bitDepth2 ||
		bps1 != bps2 ||
		fs1 != fs2 {
		err = fmt.Errorf("format of wav files are not agree")
		panic(err)
	}

	cleanAMP := goresearch.IntsToFloat64s(buf1.Data)
	noiseAMP := goresearch.IntsToFloat64s(buf2.Data)

	cleanRMS := goresearch.CalcRMS(cleanAMP)

	var start int
	var cutNoiseAmp []float64
	if len(cleanAMP) == len(noiseAMP) {
		start = 0
		cutNoiseAmp = noiseAMP[start : start+len(cleanAMP)]
	} else if len(cleanAMP) > len(noiseAMP) {
		start = rand.Intn(len(cleanAMP) - len(noiseAMP))
		cleanAMP = cleanAMP[start : start+len(cleanAMP)]
		cutNoiseAmp = noiseAMP
	} else {
		start = rand.Intn(len(noiseAMP) - len(cleanAMP))
		cutNoiseAmp = noiseAMP[start : start+len(cleanAMP)]
	}
	noiseRMS := goresearch.CalcRMS(cutNoiseAmp)
	snrList := goresearch.LinSpace(snrStart, snrEnd, snrDiv)

	var (
		adjustedNoiseAmp = make([]float64, len(cutNoiseAmp))
		mixedAmp         = make([]float64, len(cutNoiseAmp))
		fw               *os.File
		ww               *wav.Encoder
		wBuf             = new(audio.IntBuffer)
		outputName       string
		outputPath       string
	)
	wBuf.Format = &audio.Format{
		NumChannels: ch1,
		SampleRate:  fs1,
	}
	wBuf.SourceBitDepth = bitDepth1
	for _, snr := range snrList {

		adjustedNoiseRMS := goresearch.CalcAdjustedRMS(cleanRMS, snr)

		for i, v := range cutNoiseAmp {
			adjustedNoiseAmp[i] = v * (adjustedNoiseRMS / noiseRMS)
			mixedAmp[i] = cleanAMP[i] + adjustedNoiseAmp[i]
		}
		maxAmp := floats.Max(goresearch.AbsFloat64s(mixedAmp))
		if maxAmp > math.MaxInt16+1 {
			reductionRate := math.MaxInt16 / maxAmp
			for i, _ := range cutNoiseAmp {
				mixedAmp[i] *= reductionRate
			}
		}

		wBuf.Data = goresearch.Float64sToInts(mixedAmp)

		outputName, _ = goresearch.SplitPathAndExt(cleanFilepath)
		outputPath = filepath.Join(outputDir, filepath.Base(outputName)+"_snr"+strconv.Itoa(int(snr))+".wav")
		fw, err = os.Create(outputPath)
		check(err)
		ww = wav.NewEncoder(fw, fs1, bitDepth1, ch1, 1)
		err = ww.Write(wBuf)
		check(err)
		err = ww.Close()
		check(err)

	}
	fmt.Printf("\nSuccessfully created following SN Rate files!! SNR: %v\n", snrList)
}

func check(err error) {
	if err != nil {
		panic(err)
	}
}

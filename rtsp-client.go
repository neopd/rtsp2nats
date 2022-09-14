package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/aler9/gortsplib"
	"github.com/aler9/gortsplib/pkg/url"
	"github.com/nats-io/nats.go"
)

type Packet struct {
	id   int
	size int
	data []byte
}

type Stat struct {
	bytes   int
	packets int
	startMS int64
}

const (
	MAX_TXQ_COUNT int    = 11
	TX_LIMIT      uint32 = 6000
)

const (
	NAL_UNIT_TYPE_NON_IDR_SLICE byte = 1 + iota
	NAL_UNIT_TYPE_PARTITION_A
	NAL_UNIT_TYPE_PARTITION_B
	NAL_UNIT_TYPE_PARTITION_C
	NAL_UNIT_TYPE_IDR_SLICE // 5
	NAL_UNIT_TYPE_SEI
	NAL_UNIT_TYPE_SPS
	NAL_UNIT_TYPE_PPS
	NAL_UNIT_TYPE_AUD
	NAL_UNIT_TYPE_EOS
	NAL_UNIT_TYPE_FILLER_DATA
	NAL_UNIT_TYPE_SPS_EXTENSTION
	NAL_UNIT_TYPE_PREFIX_NAL_UNIT
	NAL_UNIT_TYPE_SUBSET_SPS
	NAL_UNIT_TYPE_DPS
	NAL_UNIT_TYPE_RESERVED_17
	NAL_UNIT_TYPE_RESERVED_18
	NAL_UNIT_TYPE_AUX_CODED_PICTURE
	NAL_UNIT_TYPE_DEPTH_VIEW
)

var txq chan Packet
var done chan bool
var txCount uint32

var verbose bool = false

type Param struct {
	txqCount int
	rtspURL  string
	natsAddr string
	subject  string
}

// This example shows how to connect to a RTSP server
// and read all tracks on a path.

func onPacketRTP(ctx *gortsplib.ClientOnPacketRTPCtx) {
	// log.Printf("RTP packet from track %d, payload type %d\n", ctx.TrackID, ctx.Packet.Header.PayloadType)
	if ctx.H264NALUs == nil {
		return
	}

	for _, nalu := range ctx.H264NALUs {
		// log.Printf("PayloadType: %d", ctx.Packet.Header.PayloadType)
		// log.Printf("nalu size: %d", len(nalu))
		packet := Packet{id: int(txCount), size: len(nalu), data: nalu}
		// p := ctx.Packet.Payload[:128]
		// log.Printf("\n%s", hex.Dump(p))
		naluTypeName := "Unhandled NAL Type"
		naluType := nalu[0] & 0x1f
		switch naluType {
		case NAL_UNIT_TYPE_IDR_SLICE:
			naluTypeName = "IDR_SLICE"
		case NAL_UNIT_TYPE_SPS:
			naluTypeName = "SPS"
		case NAL_UNIT_TYPE_PPS:
			naluTypeName = "PPS"
		case NAL_UNIT_TYPE_SEI:
			naluTypeName = "SEI"
		case NAL_UNIT_TYPE_NON_IDR_SLICE:
			naluTypeName = "NON_IDR_SLICE"
		}
		if verbose {
			log.Printf("%16s(%d): %d", naluTypeName, naluType, len(nalu))
		}
		// log.Printf("type: 0x%x\n", nalu[0]&0x1f)
		// log.Printf("\n%s", hex.Dump(nalu[:64]))
		txq <- packet
		txCount++
	}

	// if txCount >= TX_LIMIT {
	// 	done <- true
	// }
}

func timer(ch chan time.Time) {
	for tick := range time.Tick(time.Second * 5) {
		// fmt.Println(tick)
		ch <- tick
	}
}

func showStat(stat *Stat) {
	if stat.startMS == 0 {
		return
	}

	pid := os.Getpid()
	now := time.Now().UnixMilli()
	delta := now - stat.startMS
	bps := float32(stat.bytes*8) / (float32(delta) / 1000.0)
	log.Printf("PID: %d, Delta: %fms, Packets: %d, Bytes: %d, BPS: %f", pid, float32(delta)/1000.0, stat.packets, stat.bytes, bps)
}

func updateStat(stat *Stat) {
	now := time.Now()
	stat.startMS = now.UnixMilli()
	stat.bytes = 0
	stat.packets = 0
}

// read a packet from txq and send it to NATS
func relay(nc *nats.Conn, subject *string, stat *Stat, ch chan time.Time) {
	for {
		select {
		case packet := <-txq:
			if err := nc.Publish(*subject, packet.data); err != nil {
				log.Fatal(err)
			}
			stat.bytes += packet.size
			stat.packets++
		case <-ch:
			// log.Printf("timer tick")
			showStat(stat)
			updateStat(stat)
		case <-done:
			log.Printf("Done")
			os.Exit(0)
		}
	}
}

func main() {

	var txqCount = flag.Int("txq", MAX_TXQ_COUNT, "Max tx-q count")
	var rtspURL = flag.String("url", "", "RTSP URL")
	var natsAddr = flag.String("nats", "172.20.10.120:4222", "NATS Address")
	var subject = flag.String("subject", "area.0.cam.0.0", "Subject")
	flag.Parse()

	stat := Stat{}

	timerChan := make(chan time.Time)
	go timer(timerChan)

	nc, err := nats.Connect(*natsAddr)
	if err != nil {
		log.Fatal(err)
	}
	defer nc.Close()

	if rtspURL == nil || *rtspURL == "" {
		panic("Unknown URL")
	}

	log.Printf("txqCount: %d", *txqCount)
	txq = make(chan Packet, *txqCount)
	done = make(chan bool)

	// start relay thread
	go relay(nc, subject, &stat, timerChan)

	c := gortsplib.Client{
		// called when a RTP packet arrives
		OnPacketRTP: onPacketRTP,
		// called when a RTCP packet arrives
		OnPacketRTCP: func(ctx *gortsplib.ClientOnPacketRTCPCtx) {
			// log.Printf("RTCP packet from track %d, type %T\n", ctx.TrackID, ctx.Packet)
		},
	}

	// parse URL
	u, err := url.Parse(*rtspURL)
	if err != nil {
		panic(err)
	}

	// connect to the server
	err = c.Start(u.Scheme, u.Host)
	if err != nil {
		panic(err)
	}
	defer c.Close()

	// find published tracks
	tracks, baseURL, response, err := c.Describe(u)
	if err != nil {
		panic(err)
	}
	fmt.Println("tracks: ", tracks)
	fmt.Println("baseURL: ", baseURL)
	fmt.Println("response: ", response)

	// find the H264 track
	h264TrackID, h264track := func() (int, *gortsplib.TrackH264) {
		for i, track := range tracks {
			if h264track, ok := track.(*gortsplib.TrackH264); ok {
				return i, h264track
			}
		}
		return -1, nil
	}()

	if h264TrackID < 0 {
		log.Fatal("H264 track not found")
	}

	// if present, send SPS and PPS from the SDP to the decoder
	sps := h264track.SafeSPS()
	if sps != nil {
		log.Println("SPS found")
	}
	log.Print(sps)

	pps := h264track.SafePPS()
	if pps != nil {
		log.Println("PPS found")
	}
	log.Print(pps)

	// setup and read all tracks
	err = c.SetupAndPlay(tracks, baseURL)
	if err != nil {
		panic(err)
	}

	// wait until a fatal error
	panic(c.Wait())
}

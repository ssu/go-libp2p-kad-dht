package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/libp2p/go-libp2p"
	"github.com/multiformats/go-multiaddr"
	"github.com/peterh/liner"

	ipfs_go_log "github.com/ipfs/go-log"

	// multiaddr "github.com/multiformats/go-multiaddr"

	host "github.com/libp2p/go-libp2p-host"
	// pstore "github.com/libp2p/go-libp2p-peerstore"
	pstore2 "github.com/libp2p/go-libp2p-core/peer"

	// "github.com/anacrolix/ipfslog"

	// libp2p "github.com/libp2p/go-libp2p"
	dht "github.com/libp2p/go-libp2p-kad-dht"

	why_go_logging "github.com/whyrusleeping/go-logging"
)

func main() {
	err := errMain()
	if err != nil {
		log.Fatal(err)
	}
}

func errMain() error {
	ipfs_go_log.SetAllLoggers(ipfs_go_log.LevelWarn)
	why_go_logging.SetLevel(why_go_logging.INFO, "dht")
	// ipfslog.SetAllLoggerLevels(ipfslog.Warning)
	// ipfslog.SetModuleLevel("dht", ipfslog.Info)
	log.SetFlags(log.Flags() | log.Llongfile)
	host, err := libp2p.New(context.Background())
	if err != nil {
		return fmt.Errorf("error creating host: %s", err)
	}
	defer host.Close()
	d, err := dht.New(context.Background(), host)
	if err != nil {
		return fmt.Errorf("error creating dht node: %s", err)
	}
	defer d.Close()
	return interactiveLoop(d, host)
}

const (
	connectBootstrapNodes = "connect_bootstrap_nodes"
	bootstrapOnce         = "bootstrap_once"
	selectIndefinitely    = "select_indefinitely"
	printRoutingTable     = "print_routing_table"
	printSelfId           = "print_self_id"
	setClientMode         = "set_client_mode"
	bootstrapSelf         = "bootstrap_self"
	bootstrapRandom       = "bootstrap_random"
)

var allCommands = []string{
	connectBootstrapNodes,
	bootstrapOnce,
	selectIndefinitely,
	printRoutingTable,
	printSelfId,
	setClientMode,
	bootstrapSelf,
	bootstrapRandom,
}

func interactiveLoop(d *dht.IpfsDHT, h host.Host) error {
	s := liner.NewLiner()
	s.SetTabCompletionStyle(liner.TabPrints)
	s.SetCompleter(func(line string) (ret []string) {
		for _, c := range allCommands {
			if strings.HasPrefix(c, line) {
				ret = append(ret, c)
			}
		}
		return
	})
	defer s.Close()
	for {
		p, err := s.Prompt("> ")
		if err == io.EOF {
			return nil
		}
		if err != nil {
			panic(err)
		}
		if handleInput(p, d, h) {
			s.AppendHistory(p)
		}
	}
}

func handleInput(input string, d *dht.IpfsDHT, h host.Host) bool {
	intChan := make(chan os.Signal, 1)
	signal.Notify(intChan, os.Interrupt)
	defer signal.Stop(intChan)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		select {
		case <-intChan:
			cancel()
		case <-ctx.Done():
		}
	}()
	switch input {
	case connectBootstrapNodes:
		bootstrapNodeAddrs := dht.DefaultBootstrapPeers
		numConnected := connectToBootstrapNodes(ctx, h, bootstrapNodeAddrs)
		if numConnected == 0 {
			log.Print("failed to connect to any bootstrap nodes")
		} else {
			log.Printf("connected to %d/%d bootstrap nodes", numConnected, len(bootstrapNodeAddrs))
		}
	case bootstrapOnce:
		cfg := dht.DefaultBootstrapConfig
		//cfg.Timeout = time.Minute
		err := d.BootstrapOnce(ctx, cfg)
		if err != nil {
			log.Printf("error bootstrapping: %v", err)
		}
	case bootstrapSelf:
		log.Print(d.BootstrapSelf(ctx))
	case bootstrapRandom:
		log.Print(d.BootstrapRandom(ctx))
	case selectIndefinitely:
		<-ctx.Done()
	case printRoutingTable:
		d.RoutingTable().Print()
	case printSelfId:
		log.Printf("%s (%x)", d.PeerId().Pretty(), d.PeerKey())
	case setClientMode:
		d.SetClientMode()
	default:
		log.Printf("unknown command: %q", input)
		return false
	}
	return true
}

func connectToBootstrapNodes(ctx context.Context, h host.Host, mas []multiaddr.Multiaddr) (numConnected int32) {
	var wg sync.WaitGroup
	for _, ma := range mas {
		wg.Add(1)
		go func(ma multiaddr.Multiaddr) {
			// pi, err := pstore.InfoFromP2pAddr(ma)
			pi, err := pstore2.AddrInfoFromP2pAddr(ma)
			if err != nil {
				panic(err)
			}
			defer wg.Done()
			err = h.Connect(ctx, *pi)
			if err != nil {
				log.Printf("error connecting to bootstrap node %q: %v", ma, err)
			} else {
				atomic.AddInt32(&numConnected, 1)
			}
		}(ma)
	}
	wg.Wait()
	return
}

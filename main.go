package main

import (
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gocolly/colly"
	"github.com/logrusorgru/aurora/v3"
	"github.com/muesli/coral"
	"github.com/x6r/rp"
	"github.com/x6r/rp/rpc"
)

var (
	c  *rpc.Client
	pb playback

	id   uint64
	port uint16

	rootCmd = &coral.Command{
		Use:   "mpcrp",
		Short: "mpcrp is a cross-platform discord rich presence integration for mpc-hc",
		RunE: func(cmd *coral.Command, args []string) error {
			return start()
		},
	}
)

func init() {
	rootCmd.PersistentFlags().Uint64VarP(&id, "id", "i", 955267481772130384, "app id providing rich presence assets")
	rootCmd.PersistentFlags().Uint16VarP(&port, "port", "p", 13579, "port to connect to")

	ch := make(chan os.Signal)
	signal.Notify(ch, os.Interrupt)
	go func() {
		<-ch
		fmt.Println(aurora.Red("User interuptted! Exiting..."))
		os.Exit(0)
	}()
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(aurora.Red(err))
		os.Exit(1)
	}

	if c != nil {
		c.Logout()
	}
}

func start() error {
	var err error
	c, err = rp.NewClient(fmt.Sprintf("%d", id))
	if err != nil {
		return fmt.Errorf("Could not connect to discord rich presence client.")
	}

	go forever()
	fmt.Println(aurora.Green(fmt.Sprintf("Listening on port: %d!", port)))
	select {}
}

func forever() {
	for {
		if err := readVariables(); err != nil {
			c.ResetActivity()
			c.Logged = false
			continue
		} else if !c.Logged {
			c.Login()
		}
		updatePayload()

		time.Sleep(time.Second)
	}
}

func readVariables() error {
	uri := fmt.Sprintf("http://localhost:%d/variables.html", port)
	c := colly.NewCollector()
	c.OnHTML(".page-variables", func(e *colly.HTMLElement) {
		position, err := strconv.Atoi(e.ChildText("#position"))
		if err != nil {
			fmt.Println(aurora.Red(err))
		}
		duration, err := strconv.Atoi(e.ChildText("#duration"))
		if err != nil {
			fmt.Println(aurora.Red(err))
		}
		istate, err := strconv.Atoi(e.ChildText("#state"))
		if err != nil {
			fmt.Println(aurora.Red(err))
		}
		state := state(istate)

		pb = playback{
			file:           e.ChildText("#file"),
			state:          state,
			statestring:    e.ChildText("#statestring"),
			position:       position,
			duration:       duration,
			durationstring: e.ChildText("#durationstring"),
			version:        e.ChildText("#version"),
		}
	})
	if err := c.Visit(uri); err != nil {
		return err
	}
	return nil
}

func updatePayload() {
	activity := &rpc.Activity{
		Details:    strings.TrimSuffix(pb.file, filepath.Ext(pb.file)),
		LargeImage: "mpc-hc",
		LargeText:  "mpc-hc " + pb.version,
		SmallText:  pb.statestring,
		State:      pb.durationstring + " total",
	}

	position, duration := pb.position, pb.duration
	remaining, _ := time.ParseDuration(strconv.Itoa(duration-position) + "ms")
	start := time.Now()
	end := start.Add(remaining)

	switch pb.state {
	case paused:
		activity.SmallImage = "pause"
	case stopped:
		activity.SmallImage = "stop"
	case playing:
		activity.SmallImage = "play"
		activity.Timestamps = &rpc.Timestamps{
			Start: &start,
			End:   &end,
		}
	case idling:
		activity = &rpc.Activity{
			Details:    "idling",
			LargeImage: "mpc-hc",
			LargeText:  "mpc-hc " + pb.version,
		}
	}

	if err := c.SetActivity(activity); err != nil {
		fmt.Println(aurora.Red(err))
	}
}
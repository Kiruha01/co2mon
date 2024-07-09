package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"time"

	MQTT "github.com/eclipse/paho.mqtt.golang"
	"github.com/sstallion/go-hid"
)

var (
	temperature float32
	co2         uint16
)

func getData(dev *hid.Device) error {
	buf := make([]byte, 8)

	i, err := dev.Read(buf)
	if err != nil {
		return err
	}

	if i != len(buf) {
		return errors.New("wrong read bug size")
	}

	decode(buf)
	return nil
}

func decode(buf []byte) {

	if buf[3] != buf[0]+buf[1]+buf[2] {
		return
	}

	var w uint16
	w = uint16(buf[1])*256 + uint16(buf[2])

	switch buf[0] {
	case 0x42:
		temperature = float32(w)*0.0625 - 273.15
	case 0x50:
		co2 = w
	}
}

func sendMqtt(server, topic, user, password string) error {
	opts := MQTT.NewClientOptions()
	opts.AddBroker(server)
	opts.SetClientID("mqtt-go-sender")
	opts.SetUsername(user)
	opts.SetPassword(password)

	client := MQTT.NewClient(opts)
	if token := client.Connect(); token.Wait() && token.Error() != nil {
		return token.Error()
	}

	token := client.Publish(topic+"/temperature", 0, false, fmt.Sprintf("%.2f", temperature))
	token.Wait()
	token = client.Publish(topic+"/co2", 0, false, fmt.Sprintf("%d", co2))
	token.Wait()

	client.Disconnect(250)
	return nil
}

func main() {
	server := flag.String("server", "", "The broker URI. ex: tcp://192.168.0.1:1883")
	topic := flag.String("topic", "dadget/room", "Base topic to send to")
	user := flag.String("user", "", "The User (optional)")
	password := flag.String("password", "", "The password (optional)")

	flag.Parse()

	dev, err := hid.OpenFirst(0x04d9, 0xa052)
	if err != nil {
		fmt.Printf("can't open device - %v\n", err)
		os.Exit(1)
	}

	time.AfterFunc(time.Second*60, func() {
		println("timeout...")
		os.Exit(2)
	})

	magic := make([]byte, 8)
	dev.SendFeatureReport(magic)

	for temperature == 0 || co2 == 0 {
		if err := getData(dev); err != nil {
			fmt.Printf("error: %v\n", err)
			os.Exit(1)
		}
		fmt.Print(".")
	}

	fmt.Printf("\n\ntemp: %.2f\nco2: %d\n", temperature, co2)

	if *server != "" {
		if err := sendMqtt(*server, *topic, *user, *password); err != nil {
			fmt.Printf("error: %v\n", err)
		}
	}
}

package main

import (
	"context"
	"flag"
	"fmt"
	"time"

	"github.com/erh/vmodutils"
	sensor "go.viam.com/rdk/components/sensor"
	"go.viam.com/rdk/logging"
)

func main() {
	err := realMain()
	if err != nil {
		panic(err)
	}
}

func realMain() error {
	ctx := context.Background()
	logger := logging.NewLogger("rfid-cli")

	host := flag.String("host", "", "Machine address (required)")
	sensorName := flag.String("sensor", "rfid-reader", "Name of the RFID sensor component")
	flag.Parse()

	if *host == "" {
		return fmt.Errorf("need -host flag (get address from Viam app)")
	}

	logger.Infof("Connecting to %s...", *host)
	machine, err := vmodutils.ConnectToHostFromCLIToken(ctx, *host, logger)
	if err != nil {
		return fmt.Errorf("failed to connect: %w", err)
	}
	defer machine.Close(ctx)

	logger.Info("Connected successfully!")

	// Get the RFID sensor from the remote machine
	rfidSensor, err := sensor.FromProvider(machine, *sensorName)
	if err != nil {
		return fmt.Errorf("failed to get RFID sensor %q: %w", *sensorName, err)
	}

	fmt.Println("\nRFID sensor connected successfully!")
	fmt.Println("Hold an RFID tag near the reader to test...")
	fmt.Println("Press Ctrl+C to exit")
	fmt.Println()

	// Continuously read RFID tags
	for {
		readings, err := rfidSensor.Readings(ctx, nil)
		if err != nil {
			logger.Errorf("Error getting readings: %v", err)
			time.Sleep(500 * time.Millisecond)
			continue
		}

		// Check if a tag is present
		isPresent, ok := readings["is_tag_present"].(bool)
		if !ok {
			logger.Warn("Unexpected reading format")
			time.Sleep(500 * time.Millisecond)
			continue
		}

		if isPresent {
			// Tag detected - print the UID
			tagUID, ok := readings["tag_uid"].(string)
			if ok {
				fmt.Printf("✓ Tag detected: %s\n", tagUID)
			} else {
				fmt.Println("✓ Tag detected (UID unavailable)")
			}
		} else {
			// Check status
			status, _ := readings["status"].(string)
			if status == "error" {
				errMsg, _ := readings["error"].(string)
				fmt.Printf("✗ Error: %s\n", errMsg)
			} else {
				// No tag present - just show a dot to indicate we're polling
				fmt.Print(".")
			}
		}

		// Poll every 500ms
		time.Sleep(500 * time.Millisecond)
	}
}

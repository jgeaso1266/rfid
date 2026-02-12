package rfid

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	sensor "go.viam.com/rdk/components/sensor"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"

	"periph.io/x/conn/v3/gpio/gpioreg"
	"periph.io/x/conn/v3/spi/spireg"
	"periph.io/x/devices/v3/mfrc522"
	"periph.io/x/host/v3"
)

var (
	RfidRc522        = resource.NewModel("jalen", "rfid", "rfid-rc522")
	errUnimplemented = errors.New("unimplemented")
)

func init() {
	resource.RegisterComponent(sensor.API, RfidRc522,
		resource.Registration[sensor.Sensor, *Config]{
			Constructor: newRfidRfidRc522,
		},
	)
}

type Config struct {
	SPIBus      string `json:"spi_bus"`                // SPI bus number, e.g. "0"
	ChipSelect  string `json:"chip_select"`            // SPI chip select, e.g. "0"
	ResetPin    string `json:"reset_pin"`              // GPIO pin number, e.g. "25"
	IRQPin      string `json:"irq_pin"`                // GPIO pin number, e.g. "24"
	AntennaGain *int   `json:"antenna_gain,omitempty"` // Antenna gain 0-7, higher = longer range (default: 5)
}

// Validate ensures all parts of the config are valid and important fields exist.
// Returns three values:
//  1. Required dependencies: other resources that must exist for this resource to work.
//  2. Optional dependencies: other resources that may exist but are not required.
//  3. An error if any Config fields are missing or invalid.
//
// The `path` parameter indicates
// where this resource appears in the machine's JSON configuration
// (for example, "components.0"). You can use it in error messages
// to indicate which resource has a problem.
func (cfg *Config) Validate(path string) ([]string, []string, error) {
	if cfg.SPIBus == "" {
		return nil, nil, fmt.Errorf("%s: 'spi_bus' is required", path)
	}

	if cfg.ChipSelect == "" {
		return nil, nil, fmt.Errorf("%s: 'chip_select' is required", path)
	}

	if cfg.ResetPin == "" {
		return nil, nil, fmt.Errorf("%s: 'reset_pin' is required", path)
	}

	if cfg.IRQPin == "" {
		return nil, nil, fmt.Errorf("%s: 'irq_pin' is required", path)
	}

	if cfg.AntennaGain != nil && (*cfg.AntennaGain < 0 || *cfg.AntennaGain > 7) {
		return nil, nil, fmt.Errorf("%s: 'antenna_gain' must be between 0 and 7", path)
	}

	return nil, nil, nil
}

type rfidRfidRc522 struct {
	resource.AlwaysRebuild

	name   resource.Name
	logger logging.Logger
	cfg    *Config

	dev *mfrc522.Dev

	cancelCtx  context.Context
	cancelFunc func()
}

func newRfidRfidRc522(ctx context.Context, deps resource.Dependencies, rawConf resource.Config, logger logging.Logger) (sensor.Sensor, error) {
	conf, err := resource.NativeConfig[*Config](rawConf)
	if err != nil {
		return nil, err
	}

	return NewRfidRc522(ctx, deps, rawConf.ResourceName(), conf, logger)

}

func NewRfidRc522(ctx context.Context, deps resource.Dependencies, name resource.Name, conf *Config, logger logging.Logger) (sensor.Sensor, error) {
	// Initialize periph.io
	if _, err := host.Init(); err != nil {
		return nil, fmt.Errorf("failed to initialize periph.io: %w", err)
	}

	// Get reset GPIO pin
	resetPinName := "GPIO" + conf.ResetPin
	resetPin := gpioreg.ByName(resetPinName)
	if resetPin == nil {
		return nil, fmt.Errorf("failed to find reset pin %s", resetPinName)
	}

	// Get IRQ GPIO pin
	irqPinName := "GPIO" + conf.IRQPin
	irqPin := gpioreg.ByName(irqPinName)
	if irqPin == nil {
		return nil, fmt.Errorf("failed to find IRQ pin %s", irqPinName)
	}

	// Open SPI port
	spiPath := fmt.Sprintf("/dev/spidev%s.%s", conf.SPIBus, conf.ChipSelect)
	spiPort, err := spireg.Open(spiPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open SPI %s: %w", spiPath, err)
	}

	// Create mfrc522 device
	dev, err := mfrc522.NewSPI(spiPort, resetPin, irqPin)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize mfrc522: %w", err)
	}

	// Set antenna gain (0-7, higher = longer range, default: 5)
	gain := 5
	if conf.AntennaGain != nil {
		gain = *conf.AntennaGain
	}
	if err := dev.SetAntennaGain(gain); err != nil {
		logger.Warnf("Failed to set antenna gain to %d: %v", gain, err)
	}

	cancelCtx, cancelFunc := context.WithCancel(context.Background())

	s := &rfidRfidRc522{
		name:       name,
		logger:     logger,
		cfg:        conf,
		dev:        dev,
		cancelCtx:  cancelCtx,
		cancelFunc: cancelFunc,
	}

	logger.Infof("RFID-RC522 initialized (SPI: %s, Reset: %s, IRQ: %s)",
		spiPath, resetPinName, irqPinName)

	return s, nil
}

func (s *rfidRfidRc522) Name() resource.Name {
	return s.name
}

func (s *rfidRfidRc522) Readings(ctx context.Context, extra map[string]interface{}) (map[string]interface{}, error) {
	readings := make(map[string]interface{})

	// Read tag UID with 100ms timeout
	uid, err := s.dev.ReadUID(100 * time.Millisecond)

	if err != nil {
		// IRQ error indicates no tag is present, which is normal
		s.logger.Debugf("ReadUID error: %v", err)
		if strings.Contains(err.Error(), "IRQ error") {
			s.logger.Debug("No tag detected")
			readings["is_tag_present"] = false
			readings["status"] = "ok"
			return readings, nil
		}

		// Real error (SPI communication failure, etc.)
		s.logger.Debugf("Error reading RFID: %v", err)
		readings["is_tag_present"] = false
		readings["status"] = "error"
		readings["error"] = err.Error()
		return readings, nil
	}

	// Convert UID bytes to hex string (uppercase for readability)
	tagUID := strings.ToUpper(hex.EncodeToString(uid))

	s.logger.Debugf("Tag detected: UID=%s", tagUID)

	readings["tag_uid"] = tagUID
	readings["is_tag_present"] = true
	readings["status"] = "ok"

	return readings, nil
}

func (s *rfidRfidRc522) DoCommand(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	return nil, fmt.Errorf("not implemented")
}

func (s *rfidRfidRc522) Close(ctx context.Context) error {
	s.logger.Debug("Closing RFID-RC522")
	s.cancelFunc()

	if s.dev != nil {
		if err := s.dev.Halt(); err != nil {
			s.logger.Warnf("Error halting mfrc522: %v", err)
		}
	}

	return nil
}

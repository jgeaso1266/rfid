# RFID-RC522 Sensor Implementation Plan

**Goal:** Implement Viam sensor component for RC522 RFID reader using periph.io.

**Architecture:** Direct periph.io usage for SPI and GPIO - simple and straightforward.

**Tech Stack:** Go 1.23, Viam RDK, periph.io/x/devices/v3/mfrc522

---

## Task 1: Add Dependencies

```bash
cd /Users/jalen.geason/viam/rfid
go get periph.io/x/devices/v3/mfrc522
go get periph.io/x/conn/v3/gpio/gpioreg
go get periph.io/x/conn/v3/spi/spireg
go get periph.io/x/host/v3
go get go.viam.com/rdk@latest
go mod tidy
git init
git add go.mod go.sum
git commit -m "deps: add periph.io and Viam RDK"
```

---

## Task 2: Define Config

Update Config struct in `module.go` (lines 26-41):

```go
type Config struct {
	SPIBus     string `json:"spi_bus"`       // SPI bus number, e.g. "0"
	ChipSelect string `json:"chip_select"`   // SPI chip select, e.g. "0"
	ResetPin   string `json:"reset_pin"`     // GPIO pin number, e.g. "25"
	IRQPin     string `json:"irq_pin"`       // GPIO pin number, e.g. "24"
}
```

Update Validate method (lines 43-56):

```go
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

	return nil, nil, nil
}
```

Commit:
```bash
git add module.go
git commit -m "feat: add Config with SPI and GPIO pin numbers"
```

---

## Task 3: Add Imports and Device Field

Update imports in `module.go` (lines 3-11):

```go
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
```

Update rfidRfidRc522 struct (lines 58-68):

```go
type rfidRfidRc522 struct {
	resource.AlwaysRebuild

	name   resource.Name
	logger logging.Logger
	cfg    *Config

	dev *mfrc522.Dev

	cancelCtx  context.Context
	cancelFunc func()
}
```

Commit:
```bash
git add module.go
git commit -m "feat: add periph.io imports and device field"
```

---

## Task 4: Implement Initialization

Replace NewRfidRc522 function (lines 80-92):

```go
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

	// Set antenna gain (0-7, 5 is typical)
	if err := dev.SetAntennaGain(5); err != nil {
		logger.Warnf("Failed to set antenna gain: %v", err)
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
```

Commit:
```bash
git add module.go
git commit -m "feat: implement initialization with periph.io"
```

---

## Task 5: Implement Close and Readings

Update Close method (lines 106-110):

```go
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
```

Replace Readings method (lines 98-100):

```go
func (s *rfidRfidRc522) Readings(ctx context.Context, extra map[string]interface{}) (map[string]interface{}, error) {
	readings := make(map[string]interface{})

	// Read tag UID with 100ms timeout
	uid, err := s.dev.ReadUID(100 * time.Millisecond)

	if err != nil {
		// Check if timeout (no tag) vs real error
		if errors.Is(err, context.DeadlineExceeded) || err.Error() == "timeout" {
			s.logger.Debug("No tag detected")
			readings["is_tag_present"] = false
			readings["status"] = "ok"
			return readings, nil
		}

		// Real error
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
```

Commit:
```bash
git add module.go
git commit -m "feat: implement Close and Readings methods"
```

---

## Task 6: Update Documentation

Replace README.md:

```markdown
# [`rfid` module](https://github.com/viam-modules/rfid)

This [rfid module](https://app.viam.com/module/jalen/rfid) implements an RFID-RC522 reader using the [`rdk:component:sensor` API](https://docs.viam.com/appendix/apis/components/sensor/).

The RFID-RC522 uses SPI to communicate with the board and reads 13.56MHz RFID tags (MIFARE Classic, NTAG, etc.).

> [!NOTE]
> Before configuring your sensor, you must [create a machine](https://docs.viam.com/cloud/machines/#add-a-new-machine).

## Configure your RFID-RC522 sensor

Navigate to the [**CONFIGURE** tab](https://docs.viam.com/configure/) of your [machine](https://docs.viam.com/fleet/machines/) in the [Viam app](https://app.viam.com/).
[Add sensor / jalen:rfid:rfid-rc522 to your machine](https://docs.viam.com/configure/#components).

On the new component panel, copy and paste the following attribute template into your sensor's attributes field:

```json
{
  "spi_bus": "0",
  "chip_select": "0",
  "reset_pin": "25",
  "irq_pin": "24"
}
```

### Attributes

The following attributes are available for `jalen:rfid:rfid-rc522` sensors:

| Attribute     | Type   | Required?    | Description |
| ------------- | ------ | ------------ | ----------- |
| `spi_bus`     | string | **Required** | The SPI bus number the RC522 is connected to. Typically `"0"` on Raspberry Pi. |
| `chip_select` | string | **Required** | The SPI chip select line. `"0"` corresponds to CE0 (physical pin 24), `"1"` to CE1 (physical pin 26). |
| `reset_pin`   | string | **Required** | The GPIO pin number (BCM) connected to the RC522 RST pin. Example: `"25"` for physical pin 22. |
| `irq_pin`     | string | **Required** | The GPIO pin number (BCM) connected to the RC522 IRQ pin. Example: `"24"` for physical pin 18. |

## Wiring

Connect the RC522 module to your Raspberry Pi as follows:

| RC522 Pin | Pi Physical Pin | Pi GPIO (BCM) | Description |
|-----------|-----------------|---------------|-------------|
| SDA       | 24              | 8 (CE0)       | SPI Chip Select |
| SCK       | 23              | 11            | SPI Clock |
| MOSI      | 19              | 10            | SPI Master Out |
| MISO      | 21              | 9             | SPI Master In |
| RST       | 22              | 25            | Reset |
| IRQ       | 18              | 24            | Interrupt (optional usage) |
| GND       | Any GND         | GND           | Ground |
| 3.3V      | 1 or 17         | 3.3V          | Power (3.3V only!) |

**Important:** The RC522 operates at 3.3V. Do not connect it to 5V.

## Setup

### Enable SPI on Raspberry Pi

```bash
sudo raspi-config
```

Navigate to: **Interface Options** > **SPI** > **Enable**

Reboot if prompted, then verify SPI is enabled:

```bash
ls /dev/spidev*
```

You should see `/dev/spidev0.0` and `/dev/spidev0.1`.

## Example Configuration

```json
{
  "name": "rfid-reader",
  "model": "jalen:rfid:rfid-rc522",
  "type": "sensor",
  "namespace": "rdk",
  "attributes": {
    "spi_bus": "0",
    "chip_select": "0",
    "reset_pin": "25",
    "irq_pin": "24"
  },
  "depends_on": []
}
```

## Reading Format

The sensor returns readings in the following format:

### Tag Detected

```json
{
  "tag_uid": "A1B2C3D4",
  "is_tag_present": true,
  "status": "ok"
}
```

- `tag_uid`: The tag's unique identifier as a hex string (uppercase)
- `is_tag_present`: Boolean indicating if a tag is currently detected
- `status`: `"ok"` indicates successful read

### No Tag Present

```json
{
  "is_tag_present": false,
  "status": "ok"
}
```

Note: The `tag_uid` field is omitted when no tag is present.

### Error

```json
{
  "is_tag_present": false,
  "status": "error",
  "error": "SPI communication failed"
}
```

## Performance

- **Read latency:** ~100ms per reading
- **Detection range:** ~3cm from antenna
- **Supported tags:** MIFARE Classic, NTAG, ISO14443A compatible tags

## Troubleshooting

### "failed to open SPI"

- Verify SPI is enabled: `sudo raspi-config` → Interface Options → SPI
- Check that `/dev/spidev0.0` exists: `ls /dev/spidev*`
- Ensure your user has permission to access SPI (add to `spi` group if needed)

### "failed to find pin"

- Verify GPIO numbers in config match your wiring
- Use BCM GPIO numbers, not physical pin numbers (config uses BCM)
- Example: Physical pin 22 = GPIO 25 (BCM)

### "No tag detected"

- Hold tag within 3cm of the RC522 antenna
- Ensure 3.3V power supply is stable (check with multimeter if available)
- Verify all wiring connections, especially GND
- Try a different RFID tag to rule out damaged tags
- Check that the RC522 module LED (if present) is lit

### SPI communication errors

- Double-check SDA (chip select) wiring to physical pin 24
- Verify SCK, MOSI, MISO connections
- Ensure no loose connections or damaged wires

## Next Steps

- To test your sensor, go to the [**CONTROL** tab](https://docs.viam.com/fleet/control/).
- To write code against your sensor, use one of the [available SDKs](https://docs.viam.com/sdks/).
- To view examples using a sensor component, explore [these tutorials](https://docs.viam.com/tutorials/).
```

Commit:
```bash
git add README.md
git commit -m "docs: add clean documentation"
```

---

## Task 7: Build and Verify

```bash
make bin/rfid
make module.tar.gz
tar -tzf module.tar.gz
git log --oneline
```

---

## Success Criteria

- ✅ Compiles successfully
- ✅ Simple, direct periph.io usage
- ✅ Pin numbers in config
- ✅ No board component complexity
- ✅ No adapter code needed
- ✅ Readings returns correct format

## Next Steps

1. Deploy to Pi with RC522
2. Test with RFID tags
3. Upload to Viam registry
4. Integrate with inventory tracker

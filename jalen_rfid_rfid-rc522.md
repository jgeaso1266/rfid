# Model jalen:rfid:rfid-rc522

This sensor implements an RFID-RC522 reader using the [`rdk:component:sensor` API](https://docs.viam.com/appendix/apis/components/sensor/).

The RFID-RC522 uses SPI to communicate with the board and reads 13.56MHz RFID tags (MIFARE Classic, NTAG, etc.).

> [!NOTE]
> Before configuring your sensor, you must [create a machine](https://docs.viam.com/cloud/machines/#add-a-new-machine).

## Configuration

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

| Name            | Type   | Inclusion    | Description |
|-----------------|--------|--------------|-------------|
| `spi_bus`       | string | **Required** | The SPI bus number the RC522 is connected to. Typically `"0"` on Raspberry Pi. |
| `chip_select`   | string | **Required** | The SPI chip select line. `"0"` corresponds to CE0 (physical pin 24), `"1"` to CE1 (physical pin 26). |
| `reset_pin`     | string | **Required** | The GPIO pin number (BCM) connected to the RC522 RST pin. Example: `"25"` for physical pin 22. |
| `irq_pin`       | string | **Required** | The GPIO pin number (BCM) connected to the RC522 IRQ pin. Example: `"24"` for physical pin 18. |
| `antenna_gain`  | int    | Optional     | Antenna gain from 0-7, where higher values provide longer read range. Default: `5`. Maximum range: `7`. |

### Example Configuration

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
    "irq_pin": "24",
    "antenna_gain": 7
  },
  "depends_on": []
}
```

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

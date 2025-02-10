# [`failover` module](<https://github.com/viam-modules/failover>)

The `failover` module provides sensor models which allow you to designate a primary sensor and backup sensors in case of failure. The models implement the [`rdk:component:sensor` API](https://docs.viam.com/components/sensor/#api), [`rdk:component:movement_sensor` API](https://docs.viam.com/components/movement-sensor/#api), and [`rdk:component:power_sensor` API](https://docs.viam.com/components/power-sensor/#api) respectively.
It supports sensor, power sensor, or movement sensor models as the primary and backup components, depending on the model you configure.

> [!NOTE]
> Before configuring your sensor, you must [create a machine](https://docs.viam.com/cloud/machines/#add-a-new-machine).

Navigate to the **CONFIGURE** tab of your machine’s page in [the Viam app](https://app.viam.com/).
[Add `sensor` / `failover:sensor`, `sensor` / `failover:movement_sensor`, or `sensor` / `failover:power_sensor` to your machine](https://docs.viam.com/configure/#components).

## Configure your `failover` sensor

On the new component panel, copy and paste the following attribute template into your sensor's attributes field:

```json
{
 "primary": "sensor-primary",
  "backups": [
    "sensor-backup",
    "sensor-backup2"
  ]
}
```

Edit the attributes as applicable to the sensors you have configured.

### Attributes

The following attributes are available for all `failover` models:

| Name | Type | Required? | Description |
| ---- | ---- | --------- | ----------- |
| `primary` | string | **Required** | Name of the primary component. |
| `backups` | string array | **Required** | List of backups to use if the primary component fails. The order in the array corresponds to the order they will be used in. |

## Example configurations

### `viam:failover:sensor`
```json
  {
      "name": "my-failover-sensor",
      "model": "viam:failover:sensor",
      "type": "sensor",
      "namespace": "rdk",
      "attributes": {
        "primary": "sensor-primary",
        "backups": [
          "sensor-backup",
          "sensor-backup2"
        ]
      }
  }
```


### `viam:failover:power_sensor`
```json
  {
      "name": "my-failover-power-sensor",
      "model": "viam:failover:power_sensor",
      "type": "power_sensor",
      "namespace": "rdk",
      "attributes": {
        "primary": "primary-ps",
        "backups": [
          "backup-ps",
        ]
      }
  }
```

### `viam:failover:movement_sensor`
```json
  {
      "name": "my-failover-movement-sensor",
      "model": "viam:failover:movement_sensor",
      "type": "movement_sensor",
      "namespace": "rdk",
      "attributes": {
        "primary": "primary-ms",
        "backups": [
          "backup-ms",
        ]
      }
  }
  ```

## Next steps

- To test your sensor, movement sensor, or power sensor, expand the **TEST** section of its configuration pane or go to the [**CONTROL** tab](https://docs.viam.com/fleet/control/).
- To write code against your sensor, use one of the [available SDKs](https://docs.viam.com/sdks/).
- To view examples using a sensor component, explore [these tutorials](https://docs.viam.com/tutorials/).

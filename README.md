# [`failover` module](<https://github.com/viam-modules/failover>)

The failover module allows you to desiginate a primary and backup components in case of failure.
It supports sensor, power sensor, and movement sensor models.

### Attributes

The following attributes are available for all failover models:

| Name          | Type   | Required?    | Description                                                                                                                                                                                                                                                                                                                                                                                                                             |
| ------------- | ------ | ------------ | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `primary`  | string | **Required** | Name of the primary component.                                                                                                                                                                                                                   |
| `backups` | string array |  **Required**  | List of backups to use if the primary component fails. The order in the array corresponds to the order they will be used in.

## Example configurations

### `viam:failover:sensor`
```json
  {
      "name": "my-failover-sensor",
      "model": "viam:failover:sensor",
      "type": "sensor",
      "namespace": "viam",
      "attributes": {
        "primary": "sensor-primary",
        "backups": [
          "sensor-backup",
          "sensor-backup2"
        ]
      }
  }
```


### `viam:failover:power_sensor` <br>
```json
  {
      "name": "my-failover-power-sensor",
      "model": "viam:failover:power_sensor",
      "type": "power_sensor",
      "namespace": "viam",
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
      "namespace": "viam",
      "attributes": {
        "primary": "primary-ms",
        "backups": [
          "backup-ms",
        ]
      }
  }
  ```

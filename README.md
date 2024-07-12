# [`<failover>` module](<https://github.com/viam-modules/failover>)

The failover module allows you to desiginate a primary and backup components in case of failure.
# models
viam:failover:sensor
viam:failover:power_sensor
viam:failover:movement_sensor


### Attributes

The following attributes are available for all failover models:

| Name          | Type   | Required?    | Description                                                                                                                                                                                                                                                                                                                                                                                                                             |
| ------------- | ------ | ------------ | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `primary`  | string | **Required** | Name of the primary component.                                                                                                                                                                                                                   |
| `backups` | string array |  **Required**  | List of backups to use if the primary component fails. The order in the array corresponds to the order they will be used in.

### Example configuration:

```json
{
  "primary": "sensor1",
  "backups" : [
    "sensor2",
    "sensor3"
  ]
}
```

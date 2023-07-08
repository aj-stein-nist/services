## Configuration


`provisioning-services` is expecting to find the following top-level entries in
configuration:

- `provisioning`: provisioning service configuration. See [below](#provisioning-service-configuration).
- `vts`: Veraison Trusted Services backend configuration. See [trustedservices config](/vts/trustedservices/README.md#Configuration).

### Provisioning service configuration

- `plugin-dir`: directory that will be searched for plugin binaries.
- `listen-addr`: the address, in the form `<host>:<port>` the provisioning
  server will be listening on.

### Example

```yaml
provisioning:
  plugin-dir: ../../plugins/bin/
  listen-addr: localhost:8888
vts:
  server-addr: 127.0.0.1:50051
```

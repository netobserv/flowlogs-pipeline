## IPFIX writer

See also: [API reference](./api.md#write-ipfix).

You can configure the `write-ipfix` API to export processed data to an IPFIX collector.

If no `enterpriseId` is provided, only IPFIX standard data is exported. The `enterpriseId` must be provided, and configured on the collecting side, in order to export non-standard data such as RTT or Kubernetes names.

To date, NetObserv does not have its own PEN assigned, so it is left open for configuration. Use a number that does not conflict with a known vendor on the collecting side.

### IPFIX custom elements

<table>
  <thead>
    <tr>
      <th>Name</th>
      <th>ID</th>
      <th>Type</th>
      <th>Length</th>
    </tr>
  </thead>
  <tbody>
    <tr><td><b>sourcePodNamespace</b></td><td>7733</td><td>String</td><td>(variable)</td></tr>
    <tr><td><b>sourcePodName</b></td><td>7734</td><td>String</td><td>(variable)</td></tr>
    <tr><td><b>destinationPodNamespace</b></td><td>7735</td><td>String</td><td>(variable)</td></tr>
    <tr><td><b>destinationPodName</b></td><td>7736</td><td>String</td><td>(variable)</td></tr>
    <tr><td><b>sourceNodeName</b></td><td>7737</td><td>String</td><td>(variable)</td></tr>
    <tr><td><b>destinationNodeName</b></td><td>7738</td><td>String</td><td>(variable)</td></tr>
    <tr><td><b>timeFlowRttNs</b></td><td>7740</td><td>Unsigned64</td><td>8</td></tr>
    <tr><td><b>interfaces</b></td><td>7741</td><td>String</td><td>(variable)</td></tr>
    <tr><td><b>directions</b></td><td>7742</td><td>String</td><td>(variable)</td></tr>
    <tr><td><b>xlatSourcePort</b></td><td>7743</td><td>Unsigned16</td><td>2</td></tr>
    <tr><td><b>xlatDestinationPort</b></td><td>7744</td><td>Unsigned16</td><td>2</td></tr>
    <tr><td><b>xlatSourceIPv4Address</b></td><td>7745</td><td>Ipv4Address</td><td>4</td></tr>
    <tr><td><b>xlatDestinationIPv4Address</b></td><td>7746</td><td>Ipv4Address</td><td>4</td></tr>
    <tr><td><b>xlatSourceIPv6Address</b></td><td>7747</td><td>Ipv6Address</td><td>16</td></tr>
    <tr><td><b>xlatDestinationIPv6Address</b></td><td>7748</td><td>Ipv6Address</td><td>16</td></tr>
  </tbody>
</table>

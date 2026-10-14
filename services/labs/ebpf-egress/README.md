# ebpf-egress - catching a C2 beacon at the socket

An **eBPF** lab (`Module.EBPF`) and the only one on network telemetry. The three
other eBPF labs hook file and credential functions; this one hooks the kernel's
`tcp_connect`, so the attack is an outbound connection and the detection is a
network kprobe. The student beacons to a collector on the C2 port and reads the
flag off the egress alert the sensor posts.

## Shape

Everything is in the module's manifest bundle
(`services/backend/internal/labs/modules/ebpf-egress.yaml`), applied into the
session's disposable nested k3s cluster. All pods use the shared
`k8labs-foothold` image. The Tetragon DaemonSet + export-stdout sidecar are the
same as the other eBPF labs; only the TracingPolicy differs - here it hooks
`tcp_connect` and narrows to the C2 destination port with a `DPort` selector. The
flag rides the event's `message` (no flag file), exactly as `ebpf-detect` does.

## The setup

- `workstation` - the foothold, where the terminal attaches.
- `collector` - an `nc` listener on port `4444` behind a `collector.lab.svc`
  Service, standing in for the box an implant beacons to. It accepts the
  connection, but the detection does not depend on it: `tcp_connect` fires on the
  initiating side at connect time, so a refused or short-lived beacon is caught
  too.
- The TracingPolicy hooks `tcp_connect`, reads the `sock` arg, and matches
  `DPort == 4444`, so the cluster's ambient egress (DNS, API server, kubelet) is
  filtered out and only the beacon posts.

## Solution (spoiler)

```sh
nc -w2 collector.lab.svc 4444   # the outbound connection to :4444 is the event
# open the Defender pane -> a tcp_connect event to :4444 carrying the flag message
```

The flag is `k8labs{ebpf_catches_the_c2_beacon_at_the_socket}`. Any tool that
opens a TCP connection to port 4444 works (`curl telnet://…:4444`, a raw socket,
etc.) - the connection itself is what the kernel logs.

## The fix

Detection: instrument egress in the kernel (a `tcp_connect` feed is a full,
un-eraseable record keyed to process and destination), baseline each workload's
legitimate destinations, and alert on new ones. Prevention: a default-deny egress
`NetworkPolicy` with an explicit allow-list, so a compromised pod can only reach
what it was always meant to and the beacon never connects. Port 4444 is a
teaching stand-in for "a destination this pod should never dial"; real C2 hides
on 443 or 53, which is why baselining beats port blocklists.

---
kind: quiz
ey: GUT-CHECK
eyt: Think like the defender
tag: trick question
q: You are the defender. The beacon went to a known bad port, but real C2 rarely does. What is the durable win here?
opts:
  - t: Block port 4444 at the firewall and move on
  - t: Treat egress as telemetry - baseline normal destinations and alert on the outliers, and cut egress the workload never needs
    correct: true
  - t: Rely on netstat snapshots on a timer
  - t: Trust that the process list will show the beacon
explain: Blocking one port just moves the beacon to 443 or 53, where real C2 hides. netstat and the process list miss anything short-lived or cleaned up. The lasting value of a kernel egress hook is the full, un-eraseable feed of who connected where - so you can baseline what a workload legitimately talks to, alert on new destinations, and apply default-deny egress (NetworkPolicy) so a pod can only reach what it needs. Port 4444 was a teaching stand-in for "a destination this pod should never dial".
---

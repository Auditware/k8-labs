---
kind: quiz
ey: GUT-CHECK
eyt: Think it through
tag: trick question
q: The policy matches `tcp_connect` with destination port 4444. You open the Defender pane before beaconing. What do you see?
opts:
  - t: A flood of the cluster's DNS and API-server connections
  - t: Nothing yet - no connection to :4444 has been made, so nothing has matched
    correct: true
  - t: The flag already, because the sensor prints it on startup
  - t: An error, because port filtering is not possible in eBPF
explain: The DPort selector means only a connection to port 4444 posts an event. The cluster's ambient traffic goes to other ports (53, 443, and so on), so it is filtered out and the pane stays quiet. The flag rides the :4444 event's message, so it appears when you make the beacon - your action is what produces it, which is what makes this a task and not a freebie.
---

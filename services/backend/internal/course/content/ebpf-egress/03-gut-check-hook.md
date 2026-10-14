---
kind: quiz
ey: GUT-CHECK
eyt: Did it land?
tag: trick question
q: Your beacon connects, sends its data, and the process exits a second later. You then wipe shell history and clear netstat has nothing open. Is the connection still detectable?
opts:
  - t: No - with the process gone and history wiped, there is nothing left to find
  - t: Yes - the kernel logged the connect the instant it happened, before any of that
    correct: true
  - t: Only if the collector logged it on its side
  - t: Only if the connection stayed open long enough to appear in netstat
explain: The eBPF event was written at connect time, in the kernel, keyed to the process and destination - long before the process exited or you touched history. Wiping userspace traces and closing the socket change nothing about a record that was already made below userspace. That is the whole point of hooking the connection itself rather than its leftovers.
---

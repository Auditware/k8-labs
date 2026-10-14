---
kind: quiz
ey: GUT-CHECK
eyt: Did it land?
tag: trick question
q: You ran `env` and found a database password. Which of these did you have to compromise to get it?
opts:
  - t: Nothing - it was handed to you in the pod's own environment
    correct: true
  - t: The database, by connecting to it first
  - t: The API server, with the mounted `token`
  - t: Another pod on the network
explain: Reading `env` is not an attack - it is reading your own process. The credential was placed into the environment for you by whoever wrote the pod spec. That is exactly why it is the first thing to check - it costs nothing and often pays the most.
---

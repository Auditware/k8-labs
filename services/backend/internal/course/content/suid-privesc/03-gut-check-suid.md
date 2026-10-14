---
kind: quiz
ey: GUT-CHECK
eyt: Did it land?
tag: trick question
q: You find a `SUID`-root program you are allowed to run. What does that give you?
opts:
  - t: Root, immediately - it runs as root
  - t: Exactly what that program chooses to do as root, and nothing more
    correct: true
  - t: Nothing until you also know the root password
  - t: Read access to the file, but not the ability to run it
explain: The program runs as root, but you only get to steer it through whatever inputs it accepts - you are not handed a root shell, you are handed the program's behaviour. A careful SUID program gives you nothing extra. All the danger lives in how sloppily it treats the input you *can* control, which is the next act. And no password is involved - the SUID bit is the mechanism, not a credential.
---

---
kind: quiz
ey: GUT-CHECK
eyt: Did it land?
tag: trick question
q: The pull secret's password is stored base64-encoded, twice. What does that encoding protect it from?
opts:
  - t: Anyone without the decryption key
  - t: Nothing - base64 is reversible by anyone who can read the Secret
    correct: true
  - t: Being read by the kubelet
  - t: Network interception only
explain: base64 is an encoding, not encryption - there is no key, and reversing it is a public, one-command operation. It exists so binary and structured data survive being stored as text, not to keep anyone out. Anyone who can read the Secret can decode it completely. The only real protection for a credential like this is not letting the account read the Secret at all.
---

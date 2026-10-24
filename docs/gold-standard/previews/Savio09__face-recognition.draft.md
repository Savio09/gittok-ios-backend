# Gold-standard preview — `Savio09/face-recognition` (PARTIAL DRAFT)

> **PARTIAL — blocked on the token.** The repo's real evidence is the code in
> `face_detection.py`, which the evidence pack did **not** fetch (anonymous rate limit hit
> before it could). Everything below §3 is provisional until that file is pulled. Once a
> `GITHUB_TOKEN` is in `.env`, re-run `go run ./cmd/evidencepack Savio09/face-recognition` and
> finish this. **Track B (personal), backend/ML — README-independence case.**

## 1. Identity
- **Repo:** Savio09/face-recognition · **Kind:** ML / computer-vision script
- **Primary language:** Python · **License:** none declared
- **URL:** https://github.com/Savio09/face-recognition · default branch `main`
- Stars 1; last pushed 2026-05-08. Personal / self-directed project.

## 2. What it does
A face-detection program built with Python and OpenCV using the Haar cascade algorithm.
*Evidence:* repo description; `README.md`; `haarcascade_frontalface_default.xml` (the OpenCV
Haar cascade model file). **Confidence:** confident on the topic. **Execution:** not_executed.

## 3. How it works (from structure only — code not yet read)
- The whole project is **three files**: `README.md`, `face_detection.py`,
  `haarcascade_frontalface_default.xml`. *Evidence:* tree (3 entries).
- The `.xml` is OpenCV's pre-trained frontal-face Haar cascade; `face_detection.py` is
  presumably the script that loads it and runs detection. **Confidence:** uncertain until the
  script is read.

## 4–6. Run / contribution / tech facts — PENDING
- Needs `face_detection.py` to state: how it's run (`python face_detection.py`?), whether it
  uses a webcam vs. an image file, and the exact OpenCV API used. **No `requirements.txt`** is
  present in the tree, so the OpenCV dependency is implicit — a setup gap worth noting.
- Contribution: no LICENSE, no CONTRIBUTING → **Track B / personal**, social entry only.

## 7. Calibrated uncertainty — *this repo IS the README-independence test*
- The `README.md` is a **330-byte stub** that explains the *motivation* but gives **no run
  instructions, no dependency list, and no description of the code's behavior**. *Evidence:*
  `README.md`. A README-only preview would be nearly empty — the usable signal lives entirely
  in `face_detection.py` + the `.xml`, which is the whole point of the evidence-grounded thesis.
- **Not yet determinable:** everything behavioral, until the script is fetched.

## 8. Evidence index (so far)
`README.md` · tree (`face_detection.py`, `haarcascade_frontalface_default.xml`). **To add:**
`face_detection.py` contents.

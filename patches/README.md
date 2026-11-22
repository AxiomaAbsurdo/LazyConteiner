# Exact Linux desktop patch

`exact-linux-x11.patch` applies to Exact revision `d7b6871218430ed57c5a0fbf34860b28f523d5df`. It enables the existing winit and softbuffer X11 backends, routes a `DISPLAY` session to the desktop host, and updates the dependency lock. Wayland, headless, and explicitly selected DRM/VNC paths retain their behavior.

The previous launcher nested KWin inside X11. On this Debian desktop, KWin's initial window placement produced incorrect client pointer coordinates. A physical click at compositor position `(50, 285)` reached Exact as `(1169, 186)`, despite Exact's display mapping being valid. Moving the compositor away and back corrected the offset and restored clicks. Direct X11 avoids that compositor path rather than depending on a positioning workaround.

`exact-setup.sh` and `exact-run.sh` apply the patch idempotently. An incompatible checkout fails instead of silently ignoring the patch.

Run `python3 scripts/native-input-test.py` after a native build on an X11 desktop. It loads an isolated Contract fixture into the real native executable, sends XTest mouse/keyboard events, and checks pixels read from the native window. It verifies a button, text input, wheel scrolling, and pointer accuracy after two moves/resizes. It never accesses Docker. Screenshots and logs are saved in `.runtime/native-input/`.

The first verified run passed all input checks. Click response was at most 121 ms, including 120 ms of deliberate event pacing; this is an upper bound, not a rendering benchmark. Temporary pointer logging used during diagnosis is not part of the patch.

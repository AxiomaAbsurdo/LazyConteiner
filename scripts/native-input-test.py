#!/usr/bin/env python3
"""Exercise the actual X11 input path, not Exact's semantic agent tap API."""
import ctypes as C
import os
from pathlib import Path
import re
import subprocess
import time

ROOT = Path(__file__).resolve().parents[1]
P, U, I, L = C.c_void_p, C.c_ulong, C.c_int, C.c_long


class Data(C.Union):
    _fields_ = [('l', L * 5), ('b', C.c_char * 20)]


class Client(C.Structure):
    _fields_ = [('type', I), ('serial', U), ('send_event', I), ('display', P),
                ('window', U), ('message_type', U), ('format', I), ('data', Data)]


class Event(C.Union):
    _fields_ = [('client', Client), ('pad', L * 24)]


def library(name, declarations):
    lib = C.CDLL(name)
    for symbol, result, arguments in declarations:
        function = getattr(lib, symbol)
        function.restype, function.argtypes = result, arguments
    return lib


x = library('libX11.so.6', [
    ('XOpenDisplay', P, [C.c_char_p]), ('XCloseDisplay', I, [P]),
    ('XDefaultRootWindow', U, [P]), ('XFlush', I, [P]),
    ('XInternAtom', U, [P, C.c_char_p, I]),
    ('XSendEvent', I, [P, U, I, L, C.POINTER(Event)]),
    ('XTranslateCoordinates', I, [P, U, U, I, I, C.POINTER(I), C.POINTER(I), C.POINTER(U)]),
    ('XGetImage', P, [P, U, I, I, C.c_uint, C.c_uint, U, I]),
    ('XGetPixel', U, [P, I, I]), ('XDestroyImage', I, [P]),
    ('XStringToKeysym', U, [C.c_char_p]), ('XKeysymToKeycode', C.c_ubyte, [P, U]),
])
t = library('libXtst.so.6', [
    ('XTestFakeMotionEvent', I, [P, I, I, I, U]),
    ('XTestFakeButtonEvent', I, [P, C.c_uint, I, U]),
    ('XTestFakeKeyEvent', I, [P, C.c_uint, I, U]),
])


def wait_for(check, description, timeout=8):
    deadline = time.monotonic() + timeout
    while time.monotonic() < deadline:
        result = check()
        if result:
            return result
        time.sleep(.03)
    raise AssertionError(description)


def run():
    display = x.XOpenDisplay(None)
    if not display:
        raise SystemExit('An accessible X11 desktop and libXtst are required.')
    desktop = x.XDefaultRootWindow(display)
    output = ROOT / '.runtime' / 'native-input'
    output.mkdir(parents=True, exist_ok=True)
    plan = output / 'fixture.plan'
    with (output / 'compile.log').open('w') as log:
        subprocess.run(['bun', 'fleet/exact.mjs', 'contract', 'build',
                        '../scripts/fixtures/native-input.contract', '-o', str(plan)],
                       cwd=ROOT, stdout=log, stderr=log, check=True)
    env = dict(os.environ, EXACT_PLAN=str(plan), EXACT_SIZE='520x440')
    env.pop('WAYLAND_DISPLAY', None)
    env.pop('WAYLAND_SOCKET', None)
    binary = ROOT / 'fleet/target/host-dev/lazyconteiner-linux'
    with (output / 'host.log').open('w') as log:
        launched = time.monotonic()
        process = subprocess.Popen([str(binary)], env=env, stdout=log, stderr=log)
        try:
            def find_window():
                tree = subprocess.check_output(['xwininfo', '-root', '-tree'], text=True)
                for window in re.findall(r'(0x[0-9a-f]+).*dev\.axioma\.lazyconteiner', tree):
                    pid = subprocess.check_output(['xprop', '-id', window, '_NET_WM_PID'], text=True)
                    if pid.strip().endswith('= ' + str(process.pid)):
                        return int(window, 16)
            window = wait_for(find_window, 'Native X11 test window did not appear.')

            def message(name, values):
                event = Event()
                event.client.type, event.client.display = 33, display
                event.client.window = window
                event.client.message_type = x.XInternAtom(display, name.encode(), 0)
                event.client.format = 32
                for index, value in enumerate(values):
                    event.client.data.l[index] = value
                x.XSendEvent(display, desktop, 0, (1 << 20) | (1 << 19), C.byref(event))
                x.XFlush(display)
                time.sleep(.15)

            def pointer(px, py):
                rx, ry, child = I(), I(), U()
                assert x.XTranslateCoordinates(display, window, desktop, px, py,
                                                C.byref(rx), C.byref(ry), C.byref(child))
                assert t.XTestFakeMotionEvent(display, -1, rx.value, ry.value, 0)
                x.XFlush(display)
                time.sleep(.04)

            def click(px, py, button=1):
                pointer(px, py)
                for down in (1, 0):
                    assert t.XTestFakeButtonEvent(display, button, down, 0)
                    x.XFlush(display)
                    time.sleep(.04)

            def pixel(px, py):
                bitmap = x.XGetImage(display, window, px, py, 1, 1, U(-1).value, 2)
                assert bitmap, 'Cannot read the actual native window pixels.'
                try:
                    return x.XGetPixel(bitmap, 0, 0) & 0xFFFFFF
                finally:
                    x.XDestroyImage(bitmap)

            def color(px, py, expected, description):
                wait_for(lambda: pixel(px, py) == expected, description)

            def screenshot(name):
                subprocess.run(['import', '-window', hex(window), str(output / name)], check=True)

            message('_NET_ACTIVE_WINDOW', [2, 0])
            color(250, 30, 0xFF0000, 'Fixture initial marker missing.')
            ready = time.monotonic() - launched
            status = Path(f'/proc/{process.pid}/status').read_text()
            rss = re.search(r'^VmRSS:\s+(\d+) kB', status, re.MULTILINE).group(1)
            screenshot('before.png')
            # The empty white scroller has a thumb at its top and a track below.
            # Check before hover: indicators must remain discoverable while idle.
            pointer(350, 30)
            thumb, track = pixel(294, 160), pixel(294, 270)
            assert thumb != track, 'Overflowing list has no visible scrollbar thumb.'
            start = time.monotonic()
            click(100, 35)
            color(250, 30, 0x00FF00, 'Physical click did not activate the button.')
            elapsed = time.monotonic() - start
            click(80, 110)
            for letter in 'hello':
                key = x.XKeysymToKeycode(display, x.XStringToKeysym(letter.encode()))
                for down in (1, 0):
                    t.XTestFakeKeyEvent(display, key, down, 0)
                x.XFlush(display)
            color(250, 110, 0x0000FF, 'Physical keyboard input did not update the field.')
            for _ in range(12):
                click(100, 230, 5)
            color(294, 160, track, 'Scrollbar thumb did not leave the top after scrolling.')
            color(294, 290, thumb, 'Scrollbar thumb did not follow the scroll position.')
            click(100, 290)
            color(250, 350, 0xFFFF00, 'Wheel scrolling did not expose the clickable bottom button.')
            screenshot('after-input.png')
            pointer(294, 290)
            t.XTestFakeButtonEvent(display, 1, 1, 0)
            x.XFlush(display)
            pointer(294, 150)
            t.XTestFakeButtonEvent(display, 1, 0, 0)
            x.XFlush(display)
            pointer(350, 30)
            color(294, 160, thumb, 'Dragging the scrollbar thumb did not return to the top.')
            screenshot('after-scrollbar-drag.png')
            for index, (left, top, width, height) in enumerate([(50, 70, 720, 500), (1200, 120, 600, 460)]):
                message('_NET_MOVERESIZE_WINDOW', [(1 << 8) | (1 << 9) | (1 << 10) | (1 << 11) | (1 << 12), left, top, width, height])
                message('_NET_ACTIVE_WINDOW', [2, 0])
                click(100, 35)
                color(250, 30, 0xFF0000 if index == 0 else 0x00FF00,
                      'Click coordinates changed after moving or resizing the window.')
            screenshot('after-move-resize.png')
            print(f'PASS real X11 click, keyboard, wheel, visible scrollbar, thumb drag, move, resize; click response <= {elapsed * 1000:.0f} ms (includes input pacing).')
            print(f'Fixture ready <= {ready * 1000:.0f} ms (includes window discovery/activation); native RSS {rss} KiB.')
            print(f'Native screenshots: {output}')
        finally:
            process.terminate()
            try:
                process.wait(timeout=5)
            except subprocess.TimeoutExpired:
                process.kill()
                process.wait()
            x.XCloseDisplay(display)


if __name__ == '__main__':
    run()

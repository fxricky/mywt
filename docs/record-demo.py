#!/usr/bin/env python3
"""Drive a real mywt binary in a PTY and write an asciinema v2 cast.

The parent shell script builds mywt, creates sample repos, and exports
DEMO_HOME, DEMO_PROJECTS, and CAST_PATH. This process only records what
that binary and the shell actually print.
"""

import codecs
import json
import os
import pty
import select
import signal
import struct
import sys
import termios
import time

ROWS = 24
COLS = 112


def ioctl_winsize(fd):
    import fcntl

    fcntl.ioctl(fd, termios.TIOCSWINSZ, struct.pack("HHHH", ROWS, COLS, 0, 0))


class Recorder:
    def __init__(self, fd):
        self.fd = fd
        self.raw = bytearray()
        self.events = []
        self.start = time.monotonic()
        self.decoder = codecs.getincrementaldecoder("utf-8")("replace")
        self.closed = False

    def pump(self, timeout):
        deadline = time.monotonic() + timeout
        while time.monotonic() < deadline:
            remaining = deadline - time.monotonic()
            ready, _, _ = select.select([self.fd], [], [], min(0.1, remaining))
            if not ready:
                continue
            try:
                chunk = os.read(self.fd, 65536)
            except OSError:
                self.closed = True
                return
            if not chunk:
                self.closed = True
                return
            self.raw += chunk
            text = self.decoder.decode(chunk)
            if text:
                self.events.append((round(time.monotonic() - self.start, 3), text))

    def wait_for(self, needle, timeout):
        encoded = needle.encode()
        deadline = time.monotonic() + timeout
        while time.monotonic() < deadline:
            if encoded in self.raw:
                return
            self.pump(min(0.2, deadline - time.monotonic()))
        self.fail(f"timed out waiting for {needle!r}")

    def wait_count(self, needle, count, timeout):
        encoded = needle.encode()
        deadline = time.monotonic() + timeout
        while time.monotonic() < deadline:
            if self.raw.count(encoded) >= count:
                return
            self.pump(min(0.2, deadline - time.monotonic()))
        self.fail(f"timed out waiting for {needle!r} x{count}")

    def pause(self, seconds):
        # Keep reading so a late redraw is timestamped during the pause
        # instead of being glued to the next keystroke.
        self.pump(seconds)

    def send(self, data):
        os.write(self.fd, data)

    def type_text(self, text, delay=0.06):
        for ch in text:
            self.send(ch.encode())
            self.pause(delay)

    def enter(self):
        self.send(b"\r")

    def fail(self, message):
        text = self.raw.decode("utf-8", "replace")
        sys.stderr.write(message + "\n----- transcript -----\n" + text + "\n---------------------\n")
        raise SystemExit(1)

    def finish(self, path):
        tail = self.decoder.decode(b"", final=True)
        if tail:
            self.events.append((round(time.monotonic() - self.start, 3), tail))
        header = {
            "version": 2,
            "width": COLS,
            "height": ROWS,
            "timestamp": int(time.time()),
            "title": "mywt session picker",
            "env": {"TERM": "xterm-256color", "SHELL": "/bin/bash"},
        }
        with open(path, "w", encoding="utf-8") as out:
            out.write(json.dumps(header) + "\n")
            for stamp, text in self.events:
                out.write(json.dumps([stamp, "o", text]) + "\n")


def child(projects, env):
    ioctl_winsize(0)
    os.chdir(projects)
    os.execve("/bin/bash", ["bash", "--noprofile", "--norc", "-i"], env)


def main():
    home = os.environ["DEMO_HOME"]
    projects = os.environ["DEMO_PROJECTS"]
    cast_path = os.environ["CAST_PATH"]

    env = os.environ.copy()
    env["HOME"] = home
    env["TERM"] = "xterm-256color"
    env["PS1"] = "$ "
    env["PROMPT_COMMAND"] = ""
    env["SHELL"] = "/bin/bash"
    env["COLUMNS"] = str(COLS)
    env["LINES"] = str(ROWS)
    env.pop("LS_COLORS", None)
    for key in list(env):
        if key.startswith("MYWT_"):
            del env[key]

    pid, fd = pty.fork()
    if pid == 0:
        try:
            child(projects, env)
        except Exception as exc:  # pragma: no cover - child only
            sys.stderr.write(f"record-demo child: {exc}\n")
            os._exit(1)

    rec = Recorder(fd)
    try:
        ioctl_winsize(fd)
        rec.wait_for("$ ", 5)
        rec.pause(0.4)
        rec.type_text("pwd")
        rec.enter()
        rec.wait_for(projects, 5)
        rec.pause(0.6)

        rec.type_text("mywt")
        rec.enter()
        rec.wait_for("Select a session", 5)
        rec.wait_for("auth-refactor", 2)
        rec.wait_for("2 repositories", 2)
        rec.pause(1.6)
        rec.enter()

        rec.wait_for("mywt: auth-refactor", 5)
        rec.pause(1.0)
        rec.type_text("pwd")
        rec.enter()
        rec.wait_for(".mywt/worktree-", 5)
        rec.pause(0.5)
        rec.type_text("ls")
        rec.enter()
        rec.wait_for("backend", 5)
        rec.wait_for("frontend", 5)
        rec.pause(1.1)

        prompts = rec.raw.count(b"$ ")
        rec.type_text("exit")
        rec.enter()
        rec.wait_for("left mywt: auth-refactor", 5)
        rec.wait_for("(exit 0)", 2)
        # A new prompt means the session shell returned the terminal.
        rec.wait_count("$ ", prompts + 1, 5)
        rec.pause(0.4)
        rec.type_text("pwd")
        rec.enter()
        # The original directory was printed once before mywt. pwd prints it again.
        rec.wait_count(projects, 2, 5)
        rec.pause(1.2)
        rec.type_text("exit")
        rec.enter()
        rec.pump(0.4)

        order = [
            projects,
            "Select a session",
            "> auth-refactor",
            "mywt: auth-refactor",
            ".mywt/worktree-",
            "backend",
            "frontend",
            "left mywt: auth-refactor",
        ]
        cursor = 0
        raw = bytes(rec.raw)
        for needle in order:
            idx = raw.find(needle.encode(), cursor)
            if idx < 0:
                rec.fail(f"transcript missing {needle!r} in order")
            cursor = idx + len(needle)
        if raw.count(projects.encode()) < 2:
            rec.fail("original directory was not printed again after exit")

        rec.finish(cast_path)
    finally:
        try:
            os.kill(pid, signal.SIGKILL)
        except OSError:
            pass
        os.waitpid(pid, 0)
        os.close(fd)


if __name__ == "__main__":
    main()

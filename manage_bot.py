#!/usr/bin/env python3
"""
MTG Card Bot Manager
Unified management script for the MTG Card Discord Bot
"""

import os
import signal
import subprocess
import sys
import time
from pathlib import Path


class MTGBotManager:
    """Manager for the MTG Card Discord Bot."""

    def __init__(self):
        self.bot_process: subprocess.Popen | None = None
        self.bot_name = "MTG Card Bot"
        self.package_name = "mtg-card-bot"
        self.module_name = "mtg_card_bot"
        self.env_var = "MTG_DISCORD_TOKEN"
        self.cleanup_called = False

    def clear_screen(self) -> None:
        """Clear the terminal screen."""
        os.system("clear" if os.name == "posix" else "cls")

    def load_env_file(self) -> None:
        """Load environment variables from .env file."""
        env_file = Path(".env")
        if env_file.exists():
            print("📋 Loading environment variables from .env file...")
            with open(env_file) as f:
                for line in f:
                    line = line.strip()
                    if line and not line.startswith("#") and "=" in line:
                        key, value = line.split("=", 1)
                        os.environ[key.strip()] = value.strip().strip('"').strip("'")
            print("✅ Environment variables loaded")
        else:
            print("⚠️  No .env file found. Using system environment variables only.")

    def check_token(self) -> bool:
        """Check if the required Discord token is available."""
        return bool(os.environ.get(self.env_var))

    def check_running_processes(self) -> list[dict[str, str]]:
        """Check for running MTG bot processes."""
        try:
            # Get all processes
            result = subprocess.run(
                ["ps", "aux"], check=False, capture_output=True, text=True
            )
            processes = []

            for line in result.stdout.split("\n"):
                if (
                    any(
                        pattern in line.lower()
                        for pattern in [
                            "mtg_card_bot",
                            "mtg-card-bot",
                            "python.*mtg",
                            "uv.*mtg",
                        ]
                    )
                    and "grep" not in line
                    and "ps aux" not in line
                ):
                    parts = line.split(None, 10)
                    if len(parts) >= 11:
                        processes.append({
                            "pid": parts[1],
                            "user": parts[0],
                            "command": parts[10],
                        })
            return processes
        except Exception as e:
            print(f"⚠️  Error checking processes: {e}")
            return []

    def kill_existing_processes(self, auto_confirm: bool = False) -> bool:
        """Kill any existing MTG bot processes."""
        processes = self.check_running_processes()

        if not processes:
            print("✅ No existing MTG bot processes found")
            return True

        print(f"🔍 Found {len(processes)} existing MTG bot process(es):")
        for proc in processes:
            print(f"   PID {proc['pid']}: {proc['command'][:80]}...")

        if not auto_confirm:
            print("\n⚠️  Kill existing processes? (y/N): ", end="")
            response = input().strip().lower()
            if response not in ["y", "yes"]:
                print("❌ Operation cancelled")
                return False

        print("🛑 Terminating existing processes...")

        # Step 1: Graceful shutdown with SIGTERM
        for proc in processes:
            pid = proc["pid"]
            try:
                print(f"   Sending SIGTERM to PID {pid}...")
                os.kill(int(pid), signal.SIGTERM)
            except ProcessLookupError:
                print(f"   ✅ Process {pid} already terminated")
            except Exception as e:
                print(f"   ⚠️  Error terminating PID {pid}: {e}")

        # Step 2: Wait for graceful shutdown
        print("⏳ Waiting for graceful shutdown (5 seconds)...")
        time.sleep(5)

        # Step 3: Check what's still running and force kill if needed
        remaining_processes = self.check_running_processes()
        if remaining_processes:
            print("💀 Force killing remaining processes...")
            for proc in remaining_processes:
                pid = proc["pid"]
                try:
                    print(f"   Force killing PID {pid}...")
                    os.kill(int(pid), signal.SIGKILL)
                    time.sleep(0.5)
                except ProcessLookupError:
                    print(f"   ✅ Process {pid} already terminated")
                except Exception as e:
                    print(f"   ⚠️  Error force killing PID {pid}: {e}")

        # Step 4: Additional cleanup using pkill
        try:
            subprocess.run(
                ["pkill", "-f", "mtg.*bot"], check=False, stderr=subprocess.DEVNULL
            )
            subprocess.run(
                ["pkill", "-f", "python.*mtg"], check=False, stderr=subprocess.DEVNULL
            )
            subprocess.run(
                ["pkill", "-f", "uv.*mtg"], check=False, stderr=subprocess.DEVNULL
            )
        except Exception:
            pass  # pkill might not be available

        # Final verification
        time.sleep(1)
        final_check = self.check_running_processes()
        if final_check:
            print("⚠️  Some processes may still be running:")
            for proc in final_check:
                print(f"   PID {proc['pid']}: {proc['command'][:60]}...")
            print("   You may need to manually kill them with:")
            for proc in final_check:
                print(f"   kill -9 {proc['pid']}")
            return False
        print("✅ All processes terminated successfully")
        return True

    def start_bot(self) -> bool:
        """Start the MTG card bot."""
        # Check for existing processes
        if self.check_running_processes():
            print("⚠️  MTG bot processes are already running!")
            print("Use 'stop' or 'restart' to manage existing instances.")
            return False

        # Check for Discord token
        if not self.check_token():
            print(f"❌ Missing environment variable: {self.env_var}")
            print("Please set the Discord token in your .env file or environment")
            return False

        print(f"🚀 Starting {self.bot_name}...")

        try:
            # Use uv to run the bot
            cmd = ["uv", "run", "python", "-m", self.module_name]

            self.bot_process = subprocess.Popen(
                cmd,
                stdout=subprocess.PIPE,
                stderr=subprocess.STDOUT,
                universal_newlines=True,
                bufsize=1,
            )

            print(f"✅ {self.bot_name} started successfully!")
            print(f"   Process ID: {self.bot_process.pid}")
            print(f"   Command: {' '.join(cmd)}")
            print(
                "\nPress Ctrl+C to stop the bot, or use 'python manage_bot.py stop' from another terminal"
            )
            print("=" * 60)

            # Set up signal handlers
            signal.signal(signal.SIGINT, self._signal_handler)
            signal.signal(signal.SIGTERM, self._signal_handler)

            # Stream output
            try:
                while self.bot_process.poll() is None:
                    output = self.bot_process.stdout.readline()
                    if output:
                        print(f"[MTG BOT] {output.rstrip()}")
                    else:
                        time.sleep(0.1)

                # Process has ended
                return_code = self.bot_process.wait()
                if return_code == 0:
                    print(f"\n✅ {self.bot_name} exited normally")
                else:
                    print(f"\n❌ {self.bot_name} exited with code {return_code}")
                return return_code == 0

            except KeyboardInterrupt:
                print(f"\n🛑 Stopping {self.bot_name}...")
                self._cleanup()
                return True

        except FileNotFoundError:
            print("❌ 'uv' command not found. Please install uv first.")
            return False
        except Exception as e:
            print(f"❌ Failed to start {self.bot_name}: {e}")
            return False

    def stop_bot(self) -> bool:
        """Stop the MTG card bot."""
        print(f"🛑 Stopping {self.bot_name}...")
        return self.kill_existing_processes(auto_confirm=True)

    def restart_bot(self) -> bool:
        """Restart the MTG card bot."""
        print(f"🔄 Restarting {self.bot_name}...")

        if not self.stop_bot():
            print("❌ Failed to stop existing processes")
            return False

        print("⏳ Waiting 2 seconds before restart...")
        time.sleep(2)

        return self.start_bot()

    def status(self) -> None:
        """Show the current status of the MTG card bot."""
        print(f"📊 Status of {self.bot_name}")
        print("=" * 40)

        # Check token
        has_token = self.check_token()
        print(f"Discord Token: {'✅ Present' if has_token else '❌ Missing'}")
        if not has_token:
            print(f"   Set {self.env_var} in .env file or environment")

        # Check .env file
        env_file = Path(".env")
        print(f".env file: {'✅ Found' if env_file.exists() else '❌ Not found'}")

        # Check processes
        processes = self.check_running_processes()
        if processes:
            print(f"Running Processes: ✅ {len(processes)} found")
            for proc in processes:
                print(f"   PID {proc['pid']}: {proc['command'][:70]}...")
        else:
            print("Running Processes: ❌ None found")

        # Check pyproject.toml
        pyproject = Path("pyproject.toml")
        if pyproject.exists():
            print("Project Config: ✅ pyproject.toml found")
        else:
            print("Project Config: ❌ pyproject.toml not found")

        # Check uv installation
        try:
            result = subprocess.run(
                ["uv", "--version"], check=False, capture_output=True, text=True
            )
            if result.returncode == 0:
                version = result.stdout.strip()
                print(f"UV Package Manager: ✅ {version}")
            else:
                print("UV Package Manager: ❌ Not working properly")
        except FileNotFoundError:
            print("UV Package Manager: ❌ Not installed")

    def logs(self) -> None:
        """Show recent logs from running MTG bot processes."""
        processes = self.check_running_processes()

        if not processes:
            print("❌ No running MTG bot processes found")
            return

        print(f"📋 Monitoring logs from {len(processes)} process(es)...")
        print("Press Ctrl+C to stop monitoring")
        print("=" * 60)

        try:
            for proc in processes:
                pid = proc["pid"]
                print(f"[PID {pid}] Process: {proc['command'][:50]}...")

            print("\n⚠️  Note: This shows live output only. For historical logs,")
            print("check your bot's configured log files or system logs.")
            print("=" * 60)

            # Keep the process running to show it's monitoring
            while True:
                time.sleep(1)
                # Check if processes are still running
                current_processes = self.check_running_processes()
                if not current_processes:
                    print("\n📋 All bot processes have stopped.")
                    break

        except KeyboardInterrupt:
            print("\n👋 Stopped monitoring logs")

    def _signal_handler(self, signum: int, frame) -> None:
        """Handle shutdown signals."""
        print(f"\n🛑 Received signal {signum}, shutting down...")
        self._cleanup()

    def _cleanup(self) -> None:
        """Clean up the bot process."""
        if self.cleanup_called:
            return

        self.cleanup_called = True

        if self.bot_process and self.bot_process.poll() is None:
            print("🛑 Terminating bot process...")

            try:
                # Try graceful shutdown first
                self.bot_process.terminate()

                # Wait up to 5 seconds for graceful shutdown
                try:
                    self.bot_process.wait(timeout=5)
                    print("✅ Bot stopped gracefully")
                except subprocess.TimeoutExpired:
                    print("💀 Force killing bot process...")
                    self.bot_process.kill()
                    self.bot_process.wait()
                    print("✅ Bot force terminated")

            except Exception as e:
                print(f"⚠️  Error during cleanup: {e}")

    def show_menu(self) -> None:
        """Show the interactive menu."""
        self.clear_screen()
        print("🃏 MTG Card Bot Manager")
        print("=" * 30)
        print()
        print("Commands:")
        print("  1) start     - Start the bot")
        print("  2) stop      - Stop the bot")
        print("  3) restart   - Restart the bot")
        print("  4) status    - Show bot status")
        print("  5) logs      - Monitor bot logs")
        print("  6) kill      - Force kill all bot processes")
        print("  7) exit      - Exit manager")
        print()

    def interactive_mode(self) -> None:
        """Run in interactive mode."""
        while True:
            self.show_menu()
            choice = input("Enter your choice (1-7 or command name): ").strip().lower()
            print()

            if choice in ["1", "start"]:
                self.start_bot()
            elif choice in ["2", "stop"]:
                self.stop_bot()
            elif choice in ["3", "restart"]:
                self.restart_bot()
            elif choice in ["4", "status"]:
                self.status()
            elif choice in ["5", "logs"]:
                self.logs()
            elif choice in ["6", "kill"]:
                self.kill_existing_processes()
            elif choice in ["7", "exit", "quit", "q"]:
                print("👋 Goodbye!")
                break
            else:
                print("❌ Invalid choice. Please try again.")

            if choice not in [
                "1",
                "start",
                "5",
                "logs",
            ]:  # Don't pause for start/logs as they block
                input("\nPress Enter to continue...")

    def run(self, command: str | None = None) -> bool:
        """Run the bot manager with the given command."""
        # Load environment
        self.load_env_file()

        if not command:
            # Interactive mode
            try:
                self.interactive_mode()
                return True
            except KeyboardInterrupt:
                print("\n👋 Goodbye!")
                return True

        # Command mode
        if command == "start":
            return self.start_bot()
        if command == "stop":
            return self.stop_bot()
        if command == "restart":
            return self.restart_bot()
        if command == "status":
            self.status()
            return True
        if command == "logs":
            self.logs()
            return True
        if command == "kill":
            return self.kill_existing_processes()
        print(f"❌ Unknown command: {command}")
        print("Available commands: start, stop, restart, status, logs, kill")
        return False


def main():
    """Main entry point."""
    manager = MTGBotManager()

    if len(sys.argv) > 1:
        command = sys.argv[1].lower()
        success = manager.run(command)
        sys.exit(0 if success else 1)
    else:
        # Interactive mode
        try:
            manager.run()
        except KeyboardInterrupt:
            print("\n👋 Goodbye!")
        except Exception as e:
            print(f"❌ Unexpected error: {e}")
            sys.exit(1)


if __name__ == "__main__":
    main()

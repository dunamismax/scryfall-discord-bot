"""Simple logging functionality for MTG Card bot."""

from __future__ import annotations

import json
import logging
import sys
from typing import Any

_LOGGING_INITIALIZED = False


class Logger:
    """Light wrapper around Python's logging module for component-based logs."""

    def __init__(self, component: str = "mtg_card_bot"):
        self.component = component
        self.logger = logging.getLogger(component)

    def debug(self, message: str, **kwargs: Any) -> None:
        """Log a debug message."""
        self._log(logging.DEBUG, message, **kwargs)

    def info(self, message: str, **kwargs: Any) -> None:
        """Log an info message."""
        self._log(logging.INFO, message, **kwargs)

    def warning(self, message: str, **kwargs: Any) -> None:
        """Log a warning message."""
        self._log(logging.WARNING, message, **kwargs)

    def error(self, message: str, **kwargs: Any) -> None:
        """Log an error message."""
        self._log(logging.ERROR, message, **kwargs)

    def _log(self, level: int, message: str, **kwargs: Any) -> None:
        """Emit a structured log message with optional key/value context."""
        if kwargs:
            context = " ".join(f"{key}={value}" for key, value in kwargs.items())
            message = f"{message} {context}"
        self.logger.log(level, message)


def initialize_logger(level: str = "info", json_format: bool = False) -> None:
    """Initialize the global logger with the specified level and format."""
    global _LOGGING_INITIALIZED

    level_map = {
        "debug": logging.DEBUG,
        "info": logging.INFO,
        "warn": logging.WARNING,
        "warning": logging.WARNING,
        "error": logging.ERROR,
    }
    log_level = level_map.get(level.lower(), logging.INFO)

    if _LOGGING_INITIALIZED:
        logging.getLogger().setLevel(log_level)
        return

    if json_format:
        class JsonFormatter(logging.Formatter):
            """Formatter that outputs log records as JSON strings."""

            def format(self, record: logging.LogRecord) -> str:
                payload = {
                    "timestamp": self.formatTime(record, self.datefmt),
                    "level": record.levelname,
                    "logger": record.name,
                    "message": record.getMessage(),
                }
                return json.dumps(payload)

        handler = logging.StreamHandler(sys.stdout)
        handler.setFormatter(JsonFormatter())
        logging.basicConfig(level=log_level, handlers=[handler])
    else:
        logging.basicConfig(
            level=log_level,
            format="%(asctime)s [%(levelname)s] [%(name)s] %(message)s",
            handlers=[logging.StreamHandler(sys.stdout)],
        )
    _LOGGING_INITIALIZED = True


def with_component(component: str) -> Logger:
    """Return a logger with a component field."""
    return Logger(component)

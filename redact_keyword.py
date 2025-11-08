"""Redact a keyword from video frames by covering matches with white rectangles.

This module provides a CLI that scans each frame of an input video for occurrences
of a target keyword using Tesseract OCR. Matching regions are replaced with white
filled rectangles and the redacted video is saved while preserving the original
audio track.
"""
from __future__ import annotations

import argparse
import logging
import string
from dataclasses import dataclass
from pathlib import Path
from typing import Iterable, Tuple

import cv2
import numpy as np
from moviepy.editor import VideoFileClip
from PIL import Image
import pytesseract


LOGGER = logging.getLogger(__name__)


@dataclass
class RedactionConfig:
    """Configuration values for the redaction process."""

    keyword: str
    case_sensitive: bool = False
    min_confidence: int = 0

    def matches(self, candidate: str) -> bool:
        """Return ``True`` if *candidate* matches the configured keyword."""
        if not candidate:
            return False
        sanitized = candidate.strip(string.punctuation)
        if not sanitized:
            return False
        target = self.keyword if self.case_sensitive else self.keyword.lower()
        probe = sanitized if self.case_sensitive else sanitized.lower()
        return probe == target


@dataclass
class BoundingBox:
    """Simple bounding box representation."""

    x: int
    y: int
    width: int
    height: int

    @property
    def bottom_right(self) -> Tuple[int, int]:
        """Return the bottom-right corner of the bounding box."""
        return self.x + self.width, self.y + self.height


class FrameRedactor:
    """Redact keyword occurrences from individual video frames."""

    def __init__(self, config: RedactionConfig) -> None:
        self._config = config

    def __call__(self, frame: np.ndarray) -> np.ndarray:
        """Apply the redaction to a single frame."""
        boxes = self._find_keyword_boxes(frame)
        output = frame.copy()
        for box in boxes:
            LOGGER.debug("Redacting box at (%s, %s, %s, %s)", box.x, box.y, box.width, box.height)
            cv2.rectangle(output, (box.x, box.y), box.bottom_right, (255, 255, 255), -1)
        return output

    def _find_keyword_boxes(self, frame: np.ndarray) -> Iterable[BoundingBox]:
        """Return bounding boxes for keyword occurrences in *frame*."""
        # Tesseract expects RGB ordering; MoviePy already provides frames in RGB.
        pil_image = Image.fromarray(frame)
        data = pytesseract.image_to_data(pil_image, output_type=pytesseract.Output.DICT)
        boxes: list[BoundingBox] = []
        for text, x, y, w, h, conf in zip(
            data.get("text", []),
            data.get("left", []),
            data.get("top", []),
            data.get("width", []),
            data.get("height", []),
            data.get("conf", []),
        ):
            try:
                confidence = int(float(conf))
            except (TypeError, ValueError):
                confidence = -1
            if confidence < self._config.min_confidence:
                continue
            if self._config.matches(text.strip()):
                boxes.append(BoundingBox(int(x), int(y), int(w), int(h)))
        return boxes


def redact_video(input_path: Path, output_path: Path, config: RedactionConfig) -> None:
    """Redact occurrences of ``config.keyword`` from *input_path* into *output_path*."""
    redactor = FrameRedactor(config)
    with VideoFileClip(str(input_path)) as clip:
        LOGGER.info("Processing video: frame rate=%s, duration=%s", clip.fps, clip.duration)
        processed = clip.fl_image(redactor)
        processed.write_videofile(
            str(output_path),
            codec="libx264",
            audio_codec="aac",
            temp_audiofile=str(output_path.with_suffix(".temp-audio.m4a")),
            remove_temp=True,
        )


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("input", type=Path, help="Path to the input MP4 video")
    parser.add_argument("output", type=Path, help="Destination path for the redacted video")
    parser.add_argument("keyword", type=str, help="Keyword to redact from the video")
    parser.add_argument(
        "--case-sensitive",
        action="store_true",
        help="Match the keyword using case-sensitive comparison",
    )
    parser.add_argument(
        "--min-confidence",
        type=int,
        default=0,
        help=(
            "Only redact words whose OCR confidence is at least this value "
            "(range: 0-100)"
        ),
    )
    parser.add_argument(
        "--log-level",
        default="INFO",
        choices=["CRITICAL", "ERROR", "WARNING", "INFO", "DEBUG"],
        help="Logging verbosity",
    )
    return parser.parse_args()


def main() -> None:
    args = parse_args()
    logging.basicConfig(level=getattr(logging, args.log_level.upper()))
    config = RedactionConfig(
        keyword=args.keyword,
        case_sensitive=args.case_sensitive,
        min_confidence=max(0, min(100, args.min_confidence)),
    )
    redact_video(args.input, args.output, config)


if __name__ == "__main__":
    main()

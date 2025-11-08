from dataclasses import dataclass
from pathlib import Path
from unittest.mock import patch

import sys
import types

import pytest

PROJECT_ROOT = Path(__file__).resolve().parents[1]
if str(PROJECT_ROOT) not in sys.path:
    sys.path.insert(0, str(PROJECT_ROOT))


def _ensure_module(name: str, **attrs):
    module = types.ModuleType(name)
    for key, value in attrs.items():
        setattr(module, key, value)
    sys.modules[name] = module
    return module


if "cv2" not in sys.modules:
    _ensure_module("cv2", rectangle=lambda *args, **kwargs: None)

if "numpy" not in sys.modules:
    _ensure_module("numpy", ndarray=object)

if "pytesseract" not in sys.modules:
    pytesseract_stub = _ensure_module(
        "pytesseract",
        image_to_data=lambda *args, **kwargs: {},
        Output=types.SimpleNamespace(DICT={}),
    )
else:
    pytesseract_stub = sys.modules["pytesseract"]

if "PIL" not in sys.modules:
    pil_module = _ensure_module("PIL")
    image_module = types.ModuleType("PIL.Image")
    image_module.fromarray = lambda *args, **kwargs: None
    pil_module.Image = image_module
    sys.modules["PIL.Image"] = image_module
else:
    pil_module = sys.modules["PIL"]
    if not hasattr(pil_module, "Image"):
        image_module = types.ModuleType("PIL.Image")
        image_module.fromarray = lambda *args, **kwargs: None
        pil_module.Image = image_module
        sys.modules["PIL.Image"] = image_module

if "moviepy" not in sys.modules:
    moviepy_module = _ensure_module("moviepy")
    editor_module = types.ModuleType("moviepy.editor")
    editor_module.VideoFileClip = object  # type: ignore[assignment]
    moviepy_module.editor = editor_module
    sys.modules["moviepy.editor"] = editor_module

if not hasattr(pytesseract_stub, "Output"):
    pytesseract_stub.Output = types.SimpleNamespace(DICT={})

from redact_keyword import FrameRedactor, RedactionConfig


@dataclass
class FakeFrame:
    width: int
    height: int
    fill: int = 0

    def __post_init__(self) -> None:
        self.pixels = [
            [[self.fill, self.fill, self.fill] for _ in range(self.width)]
            for _ in range(self.height)
        ]

    def copy(self) -> "FakeFrame":
        clone = FakeFrame(self.width, self.height, self.fill)
        clone.pixels = [[pixel[:] for pixel in row] for row in self.pixels]
        return clone


@pytest.fixture(autouse=True)
def fake_image_module():
    with patch("redact_keyword.Image.fromarray", return_value=None):
        yield


def fake_rectangle(img: FakeFrame, top_left, bottom_right, color, thickness):
    if thickness != -1:
        raise AssertionError("Rectangle drawing should fill the region")
    x1, y1 = top_left
    x2, y2 = bottom_right
    for y in range(y1, min(y2, img.height)):
        for x in range(x1, min(x2, img.width)):
            img.pixels[y][x] = list(color)
    return img


@pytest.fixture(autouse=True)
def patched_rectangle():
    with patch("redact_keyword.cv2.rectangle", side_effect=fake_rectangle) as patched:
        yield patched


def make_ocr_payload(*entries):
    keys = ["text", "left", "top", "width", "height", "conf"]
    payload = {key: [] for key in keys}
    for text, left, top, width, height, conf in entries:
        payload["text"].append(text)
        payload["left"].append(left)
        payload["top"].append(top)
        payload["width"].append(width)
        payload["height"].append(height)
        payload["conf"].append(conf)
    return payload


def extract_region(frame: FakeFrame, x: int, y: int, w: int, h: int) -> list[list[list[int]]]:
    return [row[x : x + w] for row in frame.pixels[y : y + h]]


def _assert_region_color(frame: FakeFrame, x: int, y: int, w: int, h: int, expected: list[int]):
    for row in extract_region(frame, x, y, w, h):
        for pixel in row:
            assert pixel == expected


def test_redactor_applies_white_rectangle_for_keyword():
    frame = FakeFrame(50, 50)
    payload = make_ocr_payload(("secret", 10, 10, 10, 10, "90"))

    with patch("redact_keyword.pytesseract.image_to_data", return_value=payload):
        redactor = FrameRedactor(RedactionConfig(keyword="secret"))
        result = redactor(frame)

    _assert_region_color(result, 10, 10, 10, 10, [255, 255, 255])


def test_redactor_leaves_non_matching_pixels_untouched():
    frame = FakeFrame(20, 20, fill=17)
    payload = make_ocr_payload(("secret", 5, 5, 6, 4, "95"))

    with patch("redact_keyword.pytesseract.image_to_data", return_value=payload):
        redactor = FrameRedactor(RedactionConfig(keyword="secret"))
        result = redactor(frame)

    _assert_region_color(result, 5, 5, 6, 4, [255, 255, 255])

    for y in range(result.height):
        for x in range(result.width):
            if 5 <= x < 11 and 5 <= y < 9:
                continue
            assert result.pixels[y][x] == [17, 17, 17]


def test_redactor_respects_min_confidence_threshold():
    frame = FakeFrame(50, 50)
    payload = make_ocr_payload(("secret", 10, 10, 10, 10, "10"))

    with patch("redact_keyword.pytesseract.image_to_data", return_value=payload):
        redactor = FrameRedactor(RedactionConfig(keyword="secret", min_confidence=50))
        result = redactor(frame)

    region = extract_region(result, 10, 10, 10, 10)
    assert all(pixel == [0, 0, 0] for row in region for pixel in row)


def test_redactor_matches_keyword_with_trailing_punctuation():
    frame = FakeFrame(50, 50)
    payload = make_ocr_payload(("secret.", 10, 10, 10, 10, "99"))

    with patch("redact_keyword.pytesseract.image_to_data", return_value=payload):
        redactor = FrameRedactor(RedactionConfig(keyword="secret"))
        result = redactor(frame)

    region = extract_region(result, 10, 10, 10, 10)
    assert all(pixel == [255, 255, 255] for row in region for pixel in row)

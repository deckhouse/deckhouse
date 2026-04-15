from pathlib import Path
from typing import Any
from urllib.parse import urljoin

import yaml
import pytest
from playwright.sync_api import Page

_REPO_ROOT = Path(__file__).resolve().parent.parent.parent.parent
_STEPS_PATH = _REPO_ROOT / "docs/site/_data/getting_started/installer/screenshots.yaml"
SCREENSHOTS_DIR = Path(__file__).resolve().parent / "screenshots"

VIEWPORT_WIDTH = 1920
VIEWPORT_HEIGHT = 1080
WAIT_APP_SELECTOR = "#app > *"
WAIT_APP_TIMEOUT_MS = 30_000
GOTO_WAIT_UNTIL = "load"
AFTER_STEPS_WAIT = "domcontentloaded"

# После клика в SPA события load / domcontentloaded снова не приходят — ждём таймер + кадры отрисовки.
_POST_CLICK_MS = 600


def _load_config() -> dict[str, Any]:
    return yaml.safe_load(_STEPS_PATH.read_text(encoding="utf-8"))


def _flatten_screenshot_specs(config: dict[str, Any]) -> list[dict[str, Any]]:
    out: list[dict[str, Any]] = []
    for block in config["screenshots"]:
        group = block["group"]
        for item in block["items"]:
            out.append({**item, "group": group})
    return out


CONFIG = _load_config()
SCREENSHOT_SPECS = _flatten_screenshot_specs(CONFIG)


def _full_url(main_url: str, relative: str) -> str:
    if not relative:
        return main_url
    base = main_url if main_url.endswith("/") else main_url + "/"
    return urljoin(base, relative.lstrip("/"))


def _apply_click(page: Page, text: str) -> None:
    page.get_by_text(text).click()


def _wait_after_click(page: Page) -> None:
    page.wait_for_timeout(_POST_CLICK_MS)
    page.evaluate(
        """() => new Promise((resolve) => {
            requestAnimationFrame(() => requestAnimationFrame(() => resolve(undefined)));
        })"""
    )


def _apply_steps(page: Page, steps: list[Any]) -> None:
    for step in steps:
        if not isinstance(step, dict):
            continue
        if "click" in step:
            label = step["click"]
            if not isinstance(label, str):
                raise TypeError(f"steps: click ожидает строку (текст элемента), получено {type(label)!r}")
            _apply_click(page, label)
            _wait_after_click(page)


def capture_screenshot(page: Page, spec: dict[str, Any]) -> None:
    main_url = CONFIG["main_url"]
    page.set_viewport_size({"width": VIEWPORT_WIDTH, "height": VIEWPORT_HEIGHT})

    target = _full_url(main_url, spec.get("url") or "")
    page.goto(target, wait_until=GOTO_WAIT_UNTIL)
    page.wait_for_selector(
        WAIT_APP_SELECTOR,
        timeout=WAIT_APP_TIMEOUT_MS,
    )

    steps = spec.get("steps") or []
    _apply_steps(page, steps)

    page.wait_for_load_state(AFTER_STEPS_WAIT)

    SCREENSHOTS_DIR.mkdir(parents=True, exist_ok=True)
    out_path = SCREENSHOTS_DIR / spec["filename"]
    page.screenshot(path=str(out_path))


@pytest.mark.parametrize(
    "spec",
    SCREENSHOT_SPECS,
    ids=[f'{s["group"]}/{s["filename"]}' for s in SCREENSHOT_SPECS],
)
def test_screenshot_from_steps(page: Page, spec: dict[str, Any]) -> None:
    capture_screenshot(page, spec)

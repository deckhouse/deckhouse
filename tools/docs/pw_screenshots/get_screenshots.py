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
WAIT_APP_TIMEOUT_MS = 60_000
GOTO_WAIT_UNTIL = "load"
AFTER_STEPS_WAIT = "domcontentloaded"
# После появления #app и перед скриншотом — паузы для догрузки SPA (без настроек в YAML).
SETTLE_AFTER_APP_MS = 2_500
SETTLE_BEFORE_SCREENSHOT_MS = 1_500

# После клика в SPA события load / domcontentloaded снова не приходят — ждём таймер + кадры отрисовки.
_POST_CLICK_MS = 600
_WAIT_POPUP_MS = 15_000


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


def _apply_click_step(page: Page, spec: Any) -> None:
    """click: строка — сначала пункт меню (menuitem), иначе по тексту (скрытые h3 и т.п. не кликаются).
    dict: testid | selector | text (+ role, exact, nth для уточнения)."""
    if isinstance(spec, str):
        menu = page.get_by_role("menuitem", name=spec)
        if menu.count() > 0:
            menu.first.click()
        else:
            page.get_by_text(spec).first.click()
        return
    if not isinstance(spec, dict):
        raise TypeError(f"steps: click ожидает строку или dict, получено {type(spec)!r}")
    if "testid" in spec:
        tid = spec["testid"]
        if not isinstance(tid, str):
            raise TypeError("steps: click.testid ожидает строку")
        page.get_by_test_id(tid).click()
        return
    if "selector" in spec:
        sel = spec["selector"]
        if not isinstance(sel, str):
            raise TypeError("steps: click.selector ожидает строку")
        page.locator(sel).first.click()
        return
    if "text" in spec:
        text = spec["text"]
        if not isinstance(text, str):
            raise TypeError("steps: click.text ожидает строку")
        role = spec.get("role")
        exact = bool(spec.get("exact", False))
        nth = int(spec.get("nth", 0))
        if role is not None:
            if not isinstance(role, str):
                raise TypeError("steps: click.role ожидает строку")
            page.get_by_role(role, name=text, exact=exact).nth(nth).click()
        else:
            page.get_by_text(text, exact=exact).nth(nth).click()
        return
    raise ValueError("steps: click — строка или dict: testid | selector | text (+ role?, exact?, nth?)")


def _apply_hover(page: Page, text: str) -> None:
    page.get_by_text(text).first.hover()


def _apply_wait_visible(page: Page, spec: Any) -> None:
    """Дождаться появления всплывающего UI: строка — по видимому тексту; dict — selector или text."""
    if isinstance(spec, str):
        page.get_by_text(spec, exact=False).first.wait_for(state="visible", timeout=_WAIT_POPUP_MS)
        return
    if not isinstance(spec, dict):
        raise TypeError(f"steps: wait_visible ожидает строку или dict, получено {type(spec)!r}")
    if "selector" in spec:
        sel = spec["selector"]
        if not isinstance(sel, str):
            raise TypeError("steps: wait_visible.selector ожидает строку")
        page.locator(sel).first.wait_for(state="visible", timeout=_WAIT_POPUP_MS)
        return
    if "text" in spec:
        t = spec["text"]
        if not isinstance(t, str):
            raise TypeError("steps: wait_visible.text ожидает строку")
        page.get_by_text(t, exact=False).first.wait_for(state="visible", timeout=_WAIT_POPUP_MS)
        return
    raise ValueError("steps: wait_visible — укажите строку или dict с ключом selector либо text")


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
            _apply_click_step(page, step["click"])
            _wait_after_click(page)
        elif "hover" in step:
            label = step["hover"]
            if not isinstance(label, str):
                raise TypeError(f"steps: hover ожидает строку (текст элемента), получено {type(label)!r}")
            _apply_hover(page, label)
            _wait_after_click(page)
        elif "wait_visible" in step:
            _apply_wait_visible(page, step["wait_visible"])
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
    page.wait_for_timeout(SETTLE_AFTER_APP_MS)

    steps = spec.get("steps") or []
    _apply_steps(page, steps)

    page.wait_for_load_state(AFTER_STEPS_WAIT)

    page.wait_for_timeout(SETTLE_BEFORE_SCREENSHOT_MS)

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

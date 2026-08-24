#!/usr/bin/env python3
"""
discover.py — local, free project inspection. First stage of redesign-existing-site.

Reads a project's package.json and config files to infer framework, styling
system, package manager, and dev command. Does not touch the network, does
not need loomloom, does not ask the user anything this can answer itself.

Usage:
    python discover.py <project-dir> [--out discover.json]
"""

import argparse
import json
import re
import sys
from pathlib import Path

FRAMEWORK_SIGNALS = [
    ("next", ["next.config.js", "next.config.mjs", "next.config.ts"]),
    ("astro", ["astro.config.mjs", "astro.config.ts"]),
    ("remix", ["remix.config.js"]),
    ("vite", ["vite.config.js", "vite.config.ts"]),
]

STYLING_SIGNALS = [
    ("tailwind", ["tailwind.config.js", "tailwind.config.ts", "tailwind.config.cjs"]),
    ("css-modules", None),  # detected from file scan below
    ("styled-components", None),  # detected from package.json deps
]

# Vendor/build directories to skip when scanning a project's own files: an
# installed dependency can ship a stray .module.css or .html of its own,
# which would otherwise misreport the *project's* styling system, and
# scanning node_modules on any real npm project is slow for no benefit.
EXCLUDE_DIRS = {"node_modules", ".git", "dist", "build", ".next", ".astro", "out", ".output", ".turbo", ".cache"}


def _iter_files(root, pattern):
    for p in root.rglob(pattern):
        if not any(part in EXCLUDE_DIRS for part in p.relative_to(root).parts):
            yield p


def read_json(path):
    try:
        return json.loads(Path(path).read_text(encoding="utf-8"))
    except (FileNotFoundError, json.JSONDecodeError):
        return None


def detect_package_manager(root):
    if (root / "pnpm-lock.yaml").exists():
        return "pnpm"
    if (root / "yarn.lock").exists():
        return "yarn"
    if (root / "bun.lockb").exists():
        return "bun"
    if (root / "package-lock.json").exists():
        return "npm"
    return "none"


def detect_framework(root, deps):
    for name, files in FRAMEWORK_SIGNALS:
        if any((root / f).exists() for f in files):
            return name
    for dep in ("next", "astro", "@remix-run/react", "vite"):
        if dep in deps:
            return {"next": "next", "astro": "astro", "@remix-run/react": "remix", "vite": "vite"}[dep]
    if (root / "index.html").exists() and not deps:
        return "plain-html"
    return "unknown"


def detect_styling(root, deps):
    if any((root / f).exists() for f in ("tailwind.config.js", "tailwind.config.ts", "tailwind.config.cjs")):
        return "tailwind"
    if "styled-components" in deps:
        return "styled-components"
    if "@emotion/react" in deps or "@emotion/styled" in deps:
        return "emotion"
    if any(_iter_files(root, "*.module.css")):
        return "css-modules"
    if any(_iter_files(root, "*.css")) or any(
        p.suffix == ".html" and "<style>" in p.read_text(encoding="utf-8", errors="ignore")
        for p in _iter_files(root, "*.html")
        if p.stat().st_size < 2_000_000
    ):
        return "vanilla-css"
    return "unknown"


def detect_routes(root, framework):
    routes = []
    if framework in ("next", "astro", "remix"):
        pages_dirs = [root / "app", root / "pages", root / "src" / "app", root / "src" / "pages"]
        for d in pages_dirs:
            if d.exists():
                for f in d.rglob("*"):
                    if f.suffix in (".tsx", ".jsx", ".astro", ".ts", ".js") and not f.name.startswith("_"):
                        rel = f.relative_to(d).with_suffix("")
                        # Drop a trailing "index" *segment*, not any
                        # occurrence of the literal substring "index" --
                        # .replace("index", "") was a real, confirmed bug:
                        # pages/reindex.tsx became route "/re" (the "index"
                        # inside "reindex" got stripped), and
                        # pages/index-page.tsx became "/-page". Only the
                        # final path segment being exactly "index" means
                        # the framework's own index-route convention.
                        segments = str(rel).replace("\\", "/").split("/")
                        if segments and segments[-1] == "index":
                            segments = segments[:-1]
                        route = "/" + "/".join(segments)
                        routes.append(route or "/")
    elif framework == "plain-html":
        for f in root.glob("*.html"):
            routes.append("/" + f.stem if f.stem != "index" else "/")
    return sorted(set(routes)) or ["/"]


def detect_dev_command(pkg, package_manager="npm"):
    if not pkg:
        return None
    scripts = pkg.get("scripts", {})
    # runner keyed by the real detected package_manager, not hardcoded npm
    # -- a project with only pnpm-lock.yaml (no package-lock.json) was
    # correctly reported as package_manager="pnpm" but dev_command was
    # still "npm run dev", inconsistent with its own reported manager and
    # wrong if npm isn't even installed. "none" (no lockfile found) falls
    # back to npm, since `npm run` still works against a bare package.json
    # with no lockfile at all.
    runner = {"pnpm": "pnpm run", "yarn": "yarn run", "bun": "bun run", "npm": "npm run", "none": "npm run"}.get(
        package_manager, "npm run"
    )
    for key in ("dev", "start"):
        if key in scripts:
            return f"{runner} {key}"
    return None


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("project_dir")
    parser.add_argument("--out", default=".output/discover.json")
    parser.add_argument("--authority", default="leonxlnx-taste-skill")
    args = parser.parse_args()

    if args.project_dir.startswith("http://") or args.project_dir.startswith("https://"):
        sys.exit(
            "discover.py inspects a local project directory (package.json, config "
            "files), not a live URL. Point it at the checked-out source instead: "
            "mechanical-check.py and render-and-screenshot.py are the two scripts "
            "in this pipeline that take a URL directly."
        )

    root = Path(args.project_dir).resolve()
    if not root.is_dir():
        sys.exit(f"not a directory: {root}")
    pkg = read_json(root / "package.json")
    deps = {}
    if pkg:
        deps.update(pkg.get("dependencies", {}))
        deps.update(pkg.get("devDependencies", {}))

    framework = detect_framework(root, deps)
    package_manager = detect_package_manager(root)
    result = {
        "framework": framework,
        "styling_system": detect_styling(root, deps),
        "package_manager": package_manager,
        "dev_command": detect_dev_command(pkg, package_manager),
        "routes": detect_routes(root, framework),
        "existing_components_dir": next(
            (str(d.relative_to(root)) for d in [root / "src" / "components", root / "components"] if d.exists()),
            None,
        ),
        "existing_assets_dir": next(
            (str(d.relative_to(root)) for d in [root / "public", root / "assets", root / "static"] if d.exists()),
            None,
        ),
        "design_authority": args.authority,
    }

    out_path = Path(args.out)
    out_path.parent.mkdir(parents=True, exist_ok=True)
    out_path.write_text(json.dumps(result, indent=2), encoding="utf-8")
    print(json.dumps(result, indent=2))


if __name__ == "__main__":
    main()

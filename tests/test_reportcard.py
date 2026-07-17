"""Tests for reportcard/generate.py, loaded directly from its file path."""

from __future__ import annotations

import importlib.util
import json
import tempfile
import unittest
from pathlib import Path
from unittest import mock

ROOT = Path(__file__).resolve().parents[1]
SPEC = importlib.util.spec_from_file_location(
    "reportcard_generator", ROOT / "reportcard" / "generate.py"
)
assert SPEC and SPEC.loader
generator = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(generator)


class ReportCardTests(unittest.TestCase):
    """Behavioral tests for grading, report assembly, and site output."""

    def test_grade_boundaries(self) -> None:
        """Grades map to the documented score thresholds."""
        self.assertEqual(generator.grade_for(100), "A+")
        self.assertEqual(generator.grade_for(80), "B-")
        self.assertEqual(generator.grade_for(79.9), "C+")
        self.assertEqual(generator.grade_for(0), "F")

    def test_weighted_report_and_site_are_safe(self) -> None:
        """The weighted score is correct and hostile names cannot break the page."""
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            checks = {
                name: generator.result(
                    name,
                    name.title(),
                    "Description",
                    weight,
                    score,
                    "Observed",
                    [],
                    0.01,
                )
                for name, weight, score in (
                    ("format", 15, 100),
                    ("vet", 20, 100),
                    ("build", 20, 100),
                    ("tests", 30, 50),
                    ("modules", 15, 100),
                )
            }
            config = {
                "project": {
                    "name": "</script><b>unsafe</b>",
                    "tagline": "Local & static",
                },
                "quality": {"minimum_score": 80},
                "checks": {},
            }

            with (
                mock.patch.object(
                    generator, "format_check", return_value=checks["format"]
                ),
                mock.patch.object(
                    generator,
                    "command_check",
                    side_effect=lambda check_id, *_: checks[check_id],
                ),
                mock.patch.object(
                    generator, "tests_check", return_value=checks["tests"]
                ),
            ):
                report = generator.make_report(config, root)

            self.assertEqual(report["score"], 85.0)
            self.assertEqual(report["grade"], "B")
            self.assertTrue(report["passed"])

            output = root / "site"
            generator.write_site(report, output, ROOT / "reportcard")
            page = (output / "index.html").read_text(encoding="utf-8")
            embedded = page.split(
                '<script id="report-data" type="application/json">', 1
            )[1].split("</script>", 1)[0]
            self.assertNotIn("</script><b>", embedded)
            self.assertEqual(
                json.loads(embedded)["project"]["name"], "</script><b>unsafe</b>"
            )
            self.assertTrue((output / "assets" / "style.css").is_file())
            self.assertTrue((output / ".nojekyll").is_file())

    def test_source_directory_cannot_escape_repository(self) -> None:
        """A source_dir outside the repository is rejected."""
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp) / "repo"
            root.mkdir()
            config = {"project": {"source_dir": ".."}}
            with self.assertRaises(generator.ConfigurationError):
                generator.make_report(config, root)


if __name__ == "__main__":
    unittest.main()

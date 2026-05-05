#!/usr/bin/env python3
"""Fetches SonarCloud metrics and issues, then writes a formatted report."""
import os
import sys
import json
import urllib.request
import urllib.parse

TOKEN = os.environ["SONAR_TOKEN"]
PROJECT_KEY = "ElioNeto_vyx"
BASE_URL = "https://sonarcloud.io/api"


def sonar_get(path, params=None):
    url = "%s/%s" % (BASE_URL, path)
    if params:
        url += "?" + urllib.parse.urlencode(params)
    req = urllib.request.Request(url, headers={"Authorization": "Bearer " + TOKEN})
    with urllib.request.urlopen(req, timeout=30) as r:
        return json.loads(r.read())


METRIC_KEYS = (
    "bugs,vulnerabilities,code_smells,security_hotspots,"
    "coverage,duplicated_lines_density,ncloc,sqale_rating,"
    "reliability_rating,security_rating"
)
metrics_data = sonar_get("measures/component", {
    "component": PROJECT_KEY,
    "metricKeys": METRIC_KEYS,
})
measures = {
    m["metric"]: m.get("value", "N/A")
    for m in metrics_data.get("component", {}).get("measures", [])
}

all_issues = []
page = 1
while True:
    data = sonar_get("issues/search", {
        "componentKeys": PROJECT_KEY,
        "resolved": "false",
        "ps": 500,
        "p": page,
    })
    all_issues.extend(data.get("issues", []))
    paging = data.get("paging", {})
    if paging.get("pageIndex", 1) * paging.get("pageSize", 500) >= paging.get("total", 0):
        break
    page += 1

lines = []
lines.append("=" * 60)
lines.append("  SonarCloud Report -- " + PROJECT_KEY)
lines.append("=" * 60)
lines.append("")
lines.append("METRICS")
lines.append("-" * 40)

label_map = {
    "bugs": "Bugs",
    "vulnerabilities": "Vulnerabilities",
    "code_smells": "Code Smells",
    "security_hotspots": "Security Hotspots",
    "coverage": "Coverage",
    "duplicated_lines_density": "Duplicated Lines Density",
    "ncloc": "Lines of Code",
    "sqale_rating": "Maintainability Rating",
    "reliability_rating": "Reliability Rating",
    "security_rating": "Security Rating",
}

for metric_key, label in label_map.items():
    value = measures.get(metric_key, "N/A")
    lines.append("  %-35s %s" % (label + ":", value))

lines.append("")
lines.append("ISSUES (unresolved)")
lines.append("-" * 40)

severity_order = {"BLOCKER": 0, "CRITICAL": 1, "MAJOR": 2, "MINOR": 3, "INFO": 4}
sorted_issues = sorted(
    all_issues,
    key=lambda i: (severity_order.get(i.get("severity", "INFO"), 99), i.get("creationDate", "")),
)

if not sorted_issues:
    lines.append("  No unresolved issues found.")
else:
    lines.append("  Total: %d issues" % len(sorted_issues))
    lines.append("")
    for issue in sorted_issues:
        severity = issue.get("severity", "?")
        component = issue.get("component", "?").split(":")[-1]
        message = issue.get("message", "?")
        lines.append("  [%s] %s: %s" % (severity, component, message))

lines.append("")
lines.append("=" * 60)

report_path = "sonar_report.txt"
with open(report_path, "w") as f:
    f.write("\n".join(lines) + "\n")

print("Report written to %s" % report_path)
print("Total issues: %d" % len(sorted_issues))

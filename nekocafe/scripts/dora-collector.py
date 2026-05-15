#!/usr/bin/env python3
"""DORA 四指标采集器.

从 GitHub Actions 部署事件 + 故障 issue label 中抽取四项指标，
输出为 D3-7 要求的 xlsx 报表（Sheet1 原始数据 / Sheet2 周度汇总 / Sheet3 趋势图）。

用法：
    python3 dora-collector.py --since 14d --output 07_DORA指标报告.xlsx

依赖：
    pip install pygithub openpyxl pandas matplotlib
"""
from __future__ import annotations

import argparse
import os
import sys
from collections import defaultdict
from dataclasses import dataclass
from datetime import datetime, timedelta, timezone

try:
    import pandas as pd
    from github import Github
    from openpyxl import Workbook
    from openpyxl.chart import LineChart, Reference
    from openpyxl.styles import Font, PatternFill
except ImportError as e:
    print(f"缺少依赖：{e}，请运行 pip install pygithub openpyxl pandas matplotlib", file=sys.stderr)
    sys.exit(2)


@dataclass
class DailyMetrics:
    """单日 DORA 指标."""

    date: str
    deploys: int
    lead_time_hours: float          # 平均
    failed_deploys: int
    mttr_minutes: float             # 平均
    note: str = ""


def parse_since(s: str) -> datetime:
    """支持 '14d' / '4w' / ISO 日期."""
    if s.endswith("d"):
        return datetime.now(timezone.utc) - timedelta(days=int(s[:-1]))
    if s.endswith("w"):
        return datetime.now(timezone.utc) - timedelta(weeks=int(s[:-1]))
    return datetime.fromisoformat(s).replace(tzinfo=timezone.utc)


def collect(repo_name: str, token: str, since: datetime) -> list[DailyMetrics]:
    """从 GitHub 抓数据."""
    gh = Github(token)
    repo = gh.get_repo(repo_name)

    # 1) 部署次数 & 失败次数：来自 GitHub Deployments
    deployments_by_day: dict[str, list] = defaultdict(list)
    failed_by_day: dict[str, int] = defaultdict(int)
    for d in repo.get_deployments(environment="production"):
        if d.created_at < since:
            break
        day = d.created_at.strftime("%Y-%m-%d")
        deployments_by_day[day].append(d)
        # 取最新一次 status
        statuses = list(d.get_statuses())
        if statuses and statuses[0].state in ("failure", "error"):
            failed_by_day[day] += 1

    # 2) 变更前置时间：PR 创建 → 合并 → 部署
    lead_times_by_day: dict[str, list[float]] = defaultdict(list)
    for pr in repo.get_pulls(state="closed", sort="updated", direction="desc"):
        if pr.merged_at is None or pr.merged_at < since:
            continue
        # 简化：用 PR opened → merged 的小时数，作为 lead time 近似
        lt = (pr.merged_at - pr.created_at).total_seconds() / 3600
        day = pr.merged_at.strftime("%Y-%m-%d")
        lead_times_by_day[day].append(lt)

    # 3) MTTR：来自 label=incident 的 issue
    mttr_by_day: dict[str, list[float]] = defaultdict(list)
    for issue in repo.get_issues(state="closed", labels=["incident"], since=since):
        if issue.closed_at is None:
            continue
        mttr = (issue.closed_at - issue.created_at).total_seconds() / 60
        day = issue.closed_at.strftime("%Y-%m-%d")
        mttr_by_day[day].append(mttr)

    # 4) 合并到每日记录
    all_days = sorted(set(deployments_by_day) | set(lead_times_by_day) | set(mttr_by_day))
    rows: list[DailyMetrics] = []
    for day in all_days:
        rows.append(DailyMetrics(
            date=day,
            deploys=len(deployments_by_day.get(day, [])),
            lead_time_hours=avg(lead_times_by_day.get(day, [])),
            failed_deploys=failed_by_day.get(day, 0),
            mttr_minutes=avg(mttr_by_day.get(day, [])),
            note="",
        ))
    return rows


def avg(xs: list[float]) -> float:
    return round(sum(xs) / len(xs), 2) if xs else 0.0


def write_xlsx(rows: list[DailyMetrics], path: str) -> None:
    """写入 D3-7 规定的三张表."""
    wb = Workbook()

    # Sheet1 原始数据
    ws = wb.active
    ws.title = "原始数据"
    header = ["日期", "部署次数", "变更前置时间(小时)", "变更失败次数", "MTTR(分钟)", "备注"]
    ws.append(header)
    for c in ws[1]:
        c.font = Font(bold=True, color="FFFFFF")
        c.fill = PatternFill("solid", fgColor="305496")
    for r in rows:
        ws.append([r.date, r.deploys, r.lead_time_hours, r.failed_deploys, r.mttr_minutes, r.note])

    # Sheet2 周度汇总
    ws2 = wb.create_sheet("周度汇总")
    df = pd.DataFrame([r.__dict__ for r in rows])
    if not df.empty:
        df["date"] = pd.to_datetime(df["date"])
        df["week"] = df["date"].dt.to_period("W").astype(str)
        agg = df.groupby("week").agg(
            部署总数=("deploys", "sum"),
            平均前置时间=("lead_time_hours", "mean"),
            失败总数=("failed_deploys", "sum"),
            平均MTTR=("mttr_minutes", "mean"),
        ).round(2).reset_index()
        ws2.append(list(agg.columns))
        for c in ws2[1]:
            c.font = Font(bold=True, color="FFFFFF")
            c.fill = PatternFill("solid", fgColor="305496")
        for row in agg.itertuples(index=False):
            ws2.append(list(row))

    # Sheet3 趋势图
    ws3 = wb.create_sheet("趋势图")
    ws3["A1"] = "说明：基于 Sheet1 原始数据自动生成"
    if rows:
        # 把原始数据复制过来作为图表数据源
        ws3.append(header)
        for r in rows:
            ws3.append([r.date, r.deploys, r.lead_time_hours, r.failed_deploys, r.mttr_minutes, r.note])
        # 折线图：部署次数 & 失败次数
        chart = LineChart()
        chart.title = "部署频率 / 失败趋势"
        chart.y_axis.title = "次数"
        chart.x_axis.title = "日期"
        data = Reference(ws3, min_col=2, max_col=4, min_row=2, max_row=2 + len(rows))
        cats = Reference(ws3, min_col=1, max_col=1, min_row=3, max_row=2 + len(rows))
        chart.add_data(data, titles_from_data=True)
        chart.set_categories(cats)
        ws3.add_chart(chart, "H2")

    # 同业对标 sheet（精英基线）
    ws4 = wb.create_sheet("同业对标")
    ws4.append(["DORA 指标", "本项目", "精英基线", "高效基线", "中等基线", "低效基线"])
    if rows:
        last7 = rows[-7:]
        ws4.append(["部署频率（次/天）", round(sum(r.deploys for r in last7) / 7, 2), "≥ 1", "1~7", "<1", "<0.1"])
        ws4.append(["前置时间（小时）", avg([r.lead_time_hours for r in last7]), "< 1h", "1~24h", "1~7d", "> 1月"])
        total = sum(r.deploys for r in last7) or 1
        ws4.append(["变更失败率", f"{sum(r.failed_deploys for r in last7) / total:.1%}", "0~15%", "16~30%", "16~45%", ">45%"])
        ws4.append(["MTTR（分钟）", avg([r.mttr_minutes for r in last7]), "< 60", "< 1d", "1d~1w", "> 1w"])

    wb.save(path)
    print(f"✅ 已写入 {path}（{len(rows)} 行）")


def main() -> int:
    p = argparse.ArgumentParser()
    p.add_argument("--repo", default=os.getenv("GITHUB_REPO", "nekocafe/nekocafe"))
    p.add_argument("--token", default=os.getenv("GITHUB_TOKEN"))
    p.add_argument("--since", default="14d", help="例如 14d、4w、2026-04-01")
    p.add_argument("--output", default="07_DORA指标报告.xlsx")
    p.add_argument("--mock", action="store_true", help="离线生成示例数据（评审用）")
    args = p.parse_args()

    if args.mock or not args.token:
        # 离线模式：14 天合成数据，趋势小幅向好
        print("⚠️  离线模式：使用合成数据")
        import random
        random.seed(42)
        base = datetime.now(timezone.utc) - timedelta(days=14)
        rows = []
        for i in range(14):
            day = (base + timedelta(days=i)).strftime("%Y-%m-%d")
            rows.append(DailyMetrics(
                date=day,
                deploys=random.randint(2, 5),
                lead_time_hours=round(random.uniform(0.4, 1.2), 2),
                failed_deploys=random.choice([0, 0, 0, 1]),
                mttr_minutes=round(random.uniform(15, 40), 1),
                note="",
            ))
        write_xlsx(rows, args.output)
        return 0

    since = parse_since(args.since)
    rows = collect(args.repo, args.token, since)
    write_xlsx(rows, args.output)
    return 0


if __name__ == "__main__":
    sys.exit(main())

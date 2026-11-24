import argparse
import csv
import datetime as dt
import statistics
import time
from pathlib import Path
from typing import Dict, List, Optional

from hid_controller import BS2PROHIDController


def light_checksum(payload: List[int]) -> int:
    total = sum(payload[2:])
    return total & 0xFF


def send_light_command(controller: BS2PROHIDController, fields: List[int]) -> bool:
    payload = [0x5A, 0xA5, *fields]
    payload.append(light_checksum(payload))
    hex_cmd = "".join(f"{b:02x}" for b in payload)
    return controller.send_hex_command(hex_cmd)


def enter_smart_temp_mode(controller: BS2PROHIDController) -> bool:
    sequence = [
        [0x46, 0x03, 0x01],
        [0x46, 0x03, 0x01],
        [0x45, 0x02],
        [0x45, 0x03, 0x01],
        [0x44, 0x03, 0x01],
        [0x43, 0x03, 0x01],
    ]
    for fields in sequence:
        if not send_light_command(controller, fields):
            return False
        time.sleep(0.05)
    return True


def read_reports_silent(
    controller: BS2PROHIDController, duration_sec: float
) -> List[bytes]:
    if controller.device is None:
        return []

    reports: List[bytes] = []
    deadline = time.time() + duration_sec
    controller.device.set_nonblocking(True)

    while time.time() < deadline:
        data = controller.device.read(64, 50)
        if data:
            reports.append(bytes(data))
        else:
            time.sleep(0.01)

    return reports


def parse_ef_report(report: bytes) -> Optional[Dict[str, int]]:
    if len(report) < 12:
        return None
    if not (report[0] == 0x01 and report[1] == 0x5A and report[2] == 0xA5):
        return None
    if report[3] != 0xEF:
        return None

    realtime_rpm = report[8] | (report[9] << 8)
    target_rpm = report[10] | (report[11] << 8)

    return {
        "report_type": report[3],
        "status": report[4],
        "gear_mode": report[5],
        "work_mode": report[6],
        "realtime_rpm": realtime_rpm,
        "target_rpm": target_rpm,
    }


def summarize_probe_result(set_rpm: int, reports: List[bytes]) -> Dict[str, object]:
    ef_reports = [r for r in reports if parse_ef_report(r) is not None]
    parsed = [parse_ef_report(r) for r in ef_reports]
    parsed = [p for p in parsed if p is not None]

    if not parsed:
        return {
            "set_rpm": set_rpm,
            "samples": 0,
            "realtime_rpm_avg": None,
            "realtime_rpm_median": None,
            "target_rpm_avg": None,
            "target_rpm_median": None,
            "gear_mode": None,
            "work_mode": None,
            "raw_hex": "",
        }

    realtime_values = [p["realtime_rpm"] for p in parsed]
    target_values = [p["target_rpm"] for p in parsed]

    rep = ef_reports[-1]
    last = parsed[-1]

    return {
        "set_rpm": set_rpm,
        "samples": len(parsed),
        "realtime_rpm_avg": round(sum(realtime_values) / len(realtime_values), 1),
        "realtime_rpm_median": int(statistics.median(realtime_values)),
        "target_rpm_avg": round(sum(target_values) / len(target_values), 1),
        "target_rpm_median": int(statistics.median(target_values)),
        "gear_mode": last["gear_mode"],
        "work_mode": last["work_mode"],
        "raw_hex": rep.hex(),
    }


def write_outputs(rows: List[Dict[str, object]], output_prefix: Path) -> None:
    csv_path = output_prefix.with_suffix(".csv")
    md_path = output_prefix.with_suffix(".md")

    csv_fields = [
        "set_rpm",
        "samples",
        "realtime_rpm_avg",
        "realtime_rpm_median",
        "target_rpm_avg",
        "target_rpm_median",
        "gear_mode",
        "work_mode",
        "raw_hex",
    ]

    with csv_path.open("w", newline="", encoding="utf-8") as f:
        writer = csv.DictWriter(f, fieldnames=csv_fields)
        writer.writeheader()
        writer.writerows(rows)

    # 做一个“变化字节”总结，便于对照颜色/区间
    valid_hex = [bytes.fromhex(r["raw_hex"]) for r in rows if r["raw_hex"]]  # type: ignore
    changed_indices: List[int] = []
    if valid_hex:
        min_len = min(len(b) for b in valid_hex)
        for idx in range(min_len):
            vals = {b[idx] for b in valid_hex}
            if len(vals) > 1:
                changed_indices.append(idx)

    lines = [
        "# RPM -> 状态回包探针结果",
        "",
        "## 结论摘要",
        f"- 样本点数: {len(rows)}",
        f"- 有效 0xEF 报文点数: {sum(1 for r in rows if (r['samples'] or 0) > 0)}",  # type: ignore
        f"- 回包变化字节索引: {', '.join(str(i) for i in changed_indices) if changed_indices else '无'}",
        "",
        "## 逐点结果",
    ]

    for r in rows:
        lines.append(
            f"- 设定 {r['set_rpm']} RPM -> 实际中位 {r['realtime_rpm_median']} / 目标中位 {r['target_rpm_median']} / 样本 {r['samples']}"
        )

    lines.extend(
        [
            "",
            "## 字段提示",
            "- `gear_mode` 为回包偏移 5（高/低半字节混合字段）",
            "- `work_mode` 为回包偏移 6（常见 0x04/0x05）",
            "- `raw_hex` 保留整包，便于后续对照颜色状态位",
        ]
    )

    md_path.write_text("\n".join(lines), encoding="utf-8")

    print(f"已写入: {csv_path}")
    print(f"已写入: {md_path}")


def parse_rpm_points(text: str) -> List[int]:
    values: List[int] = []
    for part in text.split(","):
        part = part.strip()
        if not part:
            continue
        values.append(int(part))
    return values


def main() -> int:
    parser = argparse.ArgumentParser(
        description="探测 smart_temp 模式下 RPM 与回包状态变化关系"
    )
    parser.add_argument(
        "--rpm-points",
        default="1000,1400,1700,2000,2300,2600,2900,3200,3500,3800,4000",
        help="逗号分隔 RPM 点",
    )
    parser.add_argument(
        "--settle-sec",
        type=float,
        default=2.5,
        help="每个 RPM 点设置后等待稳定时间（秒）",
    )
    parser.add_argument(
        "--sample-sec",
        type=float,
        default=3.0,
        help="每个 RPM 点抓包时长（秒）",
    )
    parser.add_argument(
        "--vid",
        type=lambda x: int(x, 0),
        default=0x137D7,
        help="设备 VID（支持 0x 前缀）",
    )
    parser.add_argument(
        "--pid",
        type=lambda x: int(x, 0),
        default=0x1002,
        help="设备 PID（支持 0x 前缀）",
    )

    args = parser.parse_args()

    rpm_points = parse_rpm_points(args.rpm_points)
    if not rpm_points:
        print("RPM 点为空")
        return 1

    controller = BS2PROHIDController()
    if not controller.connect(args.vid, args.pid):
        print("连接设备失败")
        return 1

    try:
        if not enter_smart_temp_mode(controller):
            print("进入 smart_temp 模式失败")
            return 1

        rows: List[Dict[str, object]] = []

        for rpm in rpm_points:
            print(f"\n=== 探测 RPM: {rpm} ===")
            controller.enter_realtime_speed_mode()
            time.sleep(0.1)
            if not controller.set_fan_speed(rpm):
                print(f"设置 RPM {rpm} 失败，跳过")
                rows.append(
                    {
                        "set_rpm": rpm,
                        "samples": 0,
                        "realtime_rpm_avg": None,
                        "realtime_rpm_median": None,
                        "target_rpm_avg": None,
                        "target_rpm_median": None,
                        "gear_mode": None,
                        "work_mode": None,
                        "raw_hex": "",
                    }
                )
                continue

            time.sleep(args.settle_sec)
            reports = read_reports_silent(controller, args.sample_sec)
            row = summarize_probe_result(rpm, reports)
            rows.append(row)
            print(
                f"样本={row['samples']} 实际中位={row['realtime_rpm_median']} 目标中位={row['target_rpm_median']}"
            )

        timestamp = dt.datetime.now().strftime("%Y%m%d_%H%M%S")
        out_prefix = Path("ota") / f"rpm_rgb_probe_{timestamp}"
        write_outputs(rows, out_prefix)

        print("\n完成。请把生成的 CSV/MD 发我，我帮你反推出 RPM->颜色区间表。")
        return 0
    finally:
        controller.disconnect()


if __name__ == "__main__":
    raise SystemExit(main())

#!/usr/bin/env python3
"""
FlyDigi BS2PRO 蓝牙设备数据读取脚本
监控设备的实时转速与目标转速
"""

import asyncio
import struct
import signal
import sys
from typing import Optional
from bleak import BleakClient, BleakScanner
from bleak.backends.device import BLEDevice

# 蓝牙厂商前缀映射
VENDOR_PREFIXES = {
    "e5:66:e5": "NanjingQinhe",  # 根据数据包分析
    "00:00:00": "Unknown",
    # 可以添加更多厂商前缀
}

# 设备配置
TARGET_DEVICE_NAME = "FlyDigi BS2PRO"
# 目标特性UUID - 从设备发现中找到的自定义特性
TARGET_CHARACTERISTIC_UUID = "0000fff1-0000-1000-8000-00805f9b34fb"
SCAN_TIMEOUT = 5.0  # 扫描超时时间（秒）


class BS2PROMonitor:
    def __init__(self):
        self.client: Optional[BleakClient] = None
        self.device: Optional[BLEDevice] = None
        self.running = False

    def get_vendor_from_mac(self, mac_address: str) -> str:
        """根据MAC地址前缀获取厂商信息"""
        mac_prefix = mac_address.lower()[:8]  # 取前3字节
        return VENDOR_PREFIXES.get(mac_prefix, "Unknown")

    def parse_speed_data(self, data: bytearray) -> tuple:
        """
        解析转速数据
        根据数据包格式: 5aa5ef0b4a0705e40ce40cfb002b00000000000000000000
        8-9字节: 目标转速 (大端 uint16)
        10-11字节: 实际转速 (大端 uint16)
        """
        if len(data) < 12:
            print(f"数据包长度不足: {len(data)} 字节 (需要至少12字节)")
            return None, None

        print(f"原始数据包: {data.hex()}")

        try:
            # 提取目标转速 (字节8-9, 大端序)
            target_speed_bytes = data[8:10]
            target_speed = struct.unpack(">H", target_speed_bytes)[0]
            print(f"目标转速字节 [8-9]: {target_speed_bytes.hex()} -> {target_speed}")

            # 提取实际转速 (字节10-11, 大端序)
            actual_speed_bytes = data[10:12]
            actual_speed = struct.unpack(">H", actual_speed_bytes)[0]
            print(f"实际转速字节 [10-11]: {actual_speed_bytes.hex()} -> {actual_speed}")

            return target_speed, actual_speed
        except struct.error as e:
            print(f"数据解析错误: {e}")
            return None, None

    def notification_handler(self, characteristic, data: bytearray):
        """处理接收到的通知数据"""
        # 获取特性的handle
        handle = characteristic.handle if hasattr(characteristic, "handle") else "未知"
        uuid = (
            str(characteristic.uuid).lower()
            if hasattr(characteristic, "uuid")
            else "未知"
        )

        print("\n=== 收到通知 ===")
        print(f"特性UUID: {uuid}")
        print(f"Handle: 0x{handle:04x}")
        print(f"数据长度: {len(data)} 字节")
        print(f"原始数据: {data.hex()}")

        # 尝试解析转速数据（针对所有通知特性）
        target_speed, actual_speed = self.parse_speed_data(data)
        if target_speed is not None and actual_speed is not None:
            print(f"🎯 目标转速: {target_speed} RPM")
            print(f"⚡ 实际转速: {actual_speed} RPM")
            print(f"📊 转速差: {actual_speed - target_speed} RPM")
            print("-" * 50)
        else:
            print("❌ 无法解析为转速数据")
            print("-" * 50)

    async def scan_devices(self) -> Optional[BLEDevice]:
        """扫描蓝牙设备"""
        print(f"正在扫描蓝牙设备 ({SCAN_TIMEOUT}秒)...")

        # 获取已发现的设备
        devices = await BleakScanner.discover(timeout=SCAN_TIMEOUT)

        print(f"\n发现 {len(devices)} 个蓝牙设备:")
        print("-" * 60)

        target_device = None

        for device in devices:
            vendor = self.get_vendor_from_mac(device.address)
            device_name = device.name or "未知设备"

            # 显示设备信息
            print(f"设备名称: {device_name}")
            print(f"MAC地址: {device.address}")
            print(f"厂商: {vendor}")
            # RSSI可能不是所有平台都有
            try:
                print(f"RSSI: {getattr(device, 'rssi', '未知')} dBm")
            except Exception:
                print("RSSI: 未知")

            # 检查是否为目标设备
            if device.name == TARGET_DEVICE_NAME:
                target_device = device
                print("*** 这是目标设备 ***")

            print("-" * 60)

        if target_device:
            print(f"找到目标设备: {target_device.name} ({target_device.address})")
        else:
            print(f"未找到目标设备: {TARGET_DEVICE_NAME}")

        return target_device

    async def connect_and_monitor(self, device: BLEDevice):
        """连接设备并开始监控"""
        print(f"正在连接到设备: {device.name} ({device.address})")

        try:
            async with BleakClient(device) as client:
                self.client = client
                print(f"成功连接到 {device.name}")

                # 获取设备服务信息
                services = client.services
                service_count = (
                    len(services.services) if hasattr(services, "services") else 0
                )
                print(f"设备服务数量: {service_count}")

                # 查找所有通知特性
                notification_chars = []
                target_char = None

                for service in services:
                    print(f"\n服务: {service.uuid}")
                    for char in service.characteristics:
                        print(
                            f"  特性: {char.uuid} (Handle: 0x{char.handle:04x}) 属性: {char.properties}"
                        )
                        if "notify" in char.properties:
                            print(
                                f"  *** 找到通知特性: {char.uuid} (Handle: 0x{char.handle:04x}) ***"
                            )
                            notification_chars.append(char)
                            # 优先使用目标UUID的特性
                            if (
                                str(char.uuid).lower()
                                == TARGET_CHARACTERISTIC_UUID.lower()
                            ):
                                target_char = char
                                print("  *** 这是目标通知特性 ***")

                if notification_chars:
                    # 使用目标特性，如果没有则使用第一个可用的
                    selected_char = (
                        target_char if target_char else notification_chars[0]
                    )

                    print(
                        f"\n选择监听特性: {selected_char.uuid} (Handle: 0x{selected_char.handle:04x})"
                    )
                    await client.start_notify(selected_char, self.notification_handler)

                    # 如果有多个通知特性，也尝试监听其他的
                    other_chars = [
                        char for char in notification_chars if char != selected_char
                    ]
                    for char in other_chars[:2]:  # 最多监听额外2个特性
                        try:
                            print(
                                f"同时监听: {char.uuid} (Handle: 0x{char.handle:04x})"
                            )
                            await client.start_notify(char, self.notification_handler)
                        except Exception as e:
                            print(f"监听特性 {char.uuid} 失败: {e}")

                    print("\n🚀 监控已启动，按 Ctrl+C 退出...")
                    print("📡 等待转速数据...")
                    self.running = True

                    # 保持连接并监听数据
                    while self.running:
                        await asyncio.sleep(1)

                    # 停止所有通知
                    for char in notification_chars:
                        try:
                            await client.stop_notify(char)
                        except Exception:
                            pass
                    print("已停止监控")
                else:
                    print("未找到可用的通知特性")
                    print("设备可能不支持通知功能")

        except Exception as e:
            print(f"连接错误: {e}")

    def stop_monitoring(self):
        """停止监控"""
        self.running = False
        print("正在停止监控...")


async def main():
    """主函数"""
    monitor = BS2PROMonitor()

    # 设置信号处理器
    def signal_handler(signum, frame):
        print("\n接收到退出信号")
        monitor.stop_monitoring()

    signal.signal(signal.SIGINT, signal_handler)
    signal.signal(signal.SIGTERM, signal_handler)

    try:
        # 扫描设备
        device = await monitor.scan_devices()

        if device:
            # 连接并监控设备
            await monitor.connect_and_monitor(device)
        else:
            print("未找到目标设备，程序退出")
            return 1

    except KeyboardInterrupt:
        print("\n程序被用户中断")
    except Exception as e:
        print(f"程序执行错误: {e}")
        return 1

    return 0


if __name__ == "__main__":
    # 检查是否安装了bleak库
    try:
        import importlib.util

        if importlib.util.find_spec("bleak") is None:
            raise ImportError
        print("Bleak 库已安装")
    except ImportError:
        print("错误: 未安装bleak库")
        print("请运行: pip install bleak")
        sys.exit(1)

    # 运行主程序
    exit_code = asyncio.run(main())
    sys.exit(exit_code)

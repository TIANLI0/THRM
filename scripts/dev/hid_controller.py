import hid
import time
from typing import Optional, List, Tuple


class BS2PROHIDController:
    """BS2PRO HID 控制器"""

    def __init__(self):
        self.device = None
        self.vendor_id = None
        self.product_id = None

    def find_bs2pro_devices(self) -> List[Tuple[int, int, str]]:
        """查找所有可能的 BS2PRO HID 设备"""
        devices = []

        print("查找 BS2PRO HID 设备...")

        # 添加已知的 BS2PRO 设备信息
        known_bs2pro_devices = [
            (0x137D7, 0x1002, "FlyDigi BS2PRO"),
        ]

        # 枚举所有设备并查找匹配项
        for device_info in hid.enumerate():
            vendor_id = device_info.get("vendor_id", 0)
            product_id = device_info.get("product_id", 0)
            manufacturer = device_info.get("manufacturer_string", "")
            product_name = device_info.get("product_string", "")

            # 首先检查已知设备
            for known_vid, known_pid, known_name in known_bs2pro_devices:
                if vendor_id == known_vid and product_id == known_pid:
                    devices.append((vendor_id, product_id, known_name))
                    print(f"找到已知 BS2PRO 设备:")
                    print(f"  厂商ID: 0x{vendor_id:04X}")
                    print(f"  产品ID: 0x{product_id:04X}")
                    print(f"  制造商: {manufacturer}")
                    print(f"  产品名: {product_name}")
                    print("-" * 40)
                    continue

            # 扩展搜索条件
            search_terms = ["BS2PRO", "FLYDIGI", "FLY", "CONTROLLER", "GAMEPAD"]
            is_match = False

            for term in search_terms:
                if (
                    term in str(manufacturer).upper()
                    or term in str(product_name).upper()
                ):
                    is_match = True
                    break

            # 也检查常见的游戏手柄厂商ID
            common_gamepad_vendors = [
                0x2DC8,  # 8BitDo
                0x045E,  # Microsoft
                0x054C,  # Sony
                0x057E,  # Nintendo
                0x0F0D,  # Hori
                0x28DE,  # Valve
                0x137D7,  # FlyDigi
            ]

            if vendor_id in common_gamepad_vendors:
                is_match = True

            if is_match and (vendor_id, product_id) not in [
                (v, p) for v, p, _ in devices
            ]:
                devices.append(
                    (
                        vendor_id,
                        product_id,
                        product_name or f"Unknown-{vendor_id:04X}:{product_id:04X}",
                    )
                )
                print(f"找到潜在设备:")
                print(f"  厂商ID: 0x{vendor_id:04X}")
                print(f"  产品ID: 0x{product_id:04X}")
                print(f"  制造商: {manufacturer}")
                print(f"  产品名: {product_name}")
                print("-" * 40)

        return devices

    def connect(
        self, vendor_id: Optional[int] = None, product_id: Optional[int] = None
    ) -> bool:
        """连接到设备"""
        try:
            if vendor_id is None or product_id is None:
                # 查找潜在的 BS2PRO 设备
                devices = self.find_bs2pro_devices()
                if not devices:
                    print("未找到匹配的 HID 设备")
                    print("\n提示: 手动指定厂商ID和产品ID")
                    print("   例如: controller.connect(0x1234, 0x5678)")
                    return False

                # 尝试连接找到的每个设备
                for vid, pid, name in devices:
                    print(f"尝试连接到: {name} (0x{vid:04X}:0x{pid:04X})")
                    if self._try_connect(vid, pid):
                        return True

                print("所有潜在设备连接失败")
                return False
            else:
                return self._try_connect(vendor_id, product_id)

        except Exception as e:
            print(f"连接过程出错: {e}")
            return False

    def _try_connect(self, vendor_id: int, product_id: int) -> bool:
        """尝试连接到指定的设备"""
        try:
            self.device = hid.device()
            self.device.open(vendor_id, product_id)
            self.vendor_id = vendor_id
            self.product_id = product_id

            # 获取设备信息
            manufacturer = self.device.get_manufacturer_string() or "Unknown"
            product = self.device.get_product_string() or "Unknown"

            print(f"连接成功!")
            print(f"  制造商: {manufacturer}")
            print(f"  产品: {product}")
            print(f"  厂商ID: 0x{vendor_id:04X}")
            print(f"  产品ID: 0x{product_id:04X}")

            return True

        except Exception as e:
            print(f"连接到 0x{vendor_id:04X}:0x{product_id:04X} 失败: {e}")
            if self.device:
                try:
                    self.device.close()
                except:
                    pass
                self.device = None
            return False

    def send_feature_report(self, report_id: int, data: bytes) -> bool:
        """发送特性报告"""
        try:
            if not self.device:
                print("设备未连接")
                return False

            # 构造报告（报告ID + 数据）
            report = bytes([report_id]) + data
            result = self.device.send_feature_report(report)
            print(f"发送特性报告成功: 报告ID={report_id}, 长度={result}")
            return True

        except Exception as e:
            print(f"发送特性报告失败: {e}")
            return False

    def get_feature_report(self, report_id: int, length: int = 64) -> Optional[bytes]:
        """获取特性报告"""
        try:
            if not self.device:
                print("设备未连接")
                return None

            report = self.device.get_feature_report(report_id, length)
            print(f"接收特性报告: 报告ID={report_id}, 数据={bytes(report).hex()}")
            return bytes(report)

        except Exception as e:
            print(f"获取特性报告失败: {e}")
            return None

    def send_output_report(self, data: bytes) -> bool:
        """发送输出报告"""
        try:
            if not self.device:
                print("设备未连接")
                return False

            result = self.device.write(data)
            print(f"发送输出报告成功: 长度={result}")
            return True

        except Exception as e:
            print(f"发送输出报告失败: {e}")
            return False

    def read_input_report(self, timeout: int = 1000) -> Optional[bytes]:
        """读取输入报告"""
        try:
            if not self.device:
                print("设备未连接")
                return None

            # 设置非阻塞模式
            self.device.set_nonblocking(True)
            data = self.device.read(64, timeout)

            if data:
                print(f"接收输入报告: 数据={bytes(data).hex()}")
                return bytes(data)
            else:
                print("未接收到输入报告（超时或无数据）")
                return None

        except Exception as e:
            print(f"读取输入报告失败: {e}")
            return None

    def send_hex_command(
        self, hex_string: str, report_id: int = 0x02, padding_length: int = 23
    ) -> bool:
        """发送十六进制命令

        Args:
            hex_string: 十六进制字符串 (如: "5aa5410243")
            report_id: 报告ID
            padding_length: 总长度，不足时用0填充
        """
        try:
            # 去除空格并转换为bytes
            hex_string = hex_string.replace(" ", "").replace("0x", "")
            payload = bytes.fromhex(hex_string)

            # 计算需要填充的长度（总长度 - 报告ID(1字节) - 有效载荷长度）
            padding_needed = padding_length - 1 - len(payload)
            if padding_needed < 0:
                print(
                    f"警告: 命令长度({len(payload)})超过最大允许长度({padding_length-1})"
                )
                padding_needed = 0

            padding = bytes(padding_needed)
            command = bytes([report_id]) + payload + padding

            print(f"发送命令: {hex_string} (总长度: {len(command)})")
            return self.send_output_report(command)

        except ValueError as e:
            print(f"十六进制格式错误: {e}")
            return False
        except Exception as e:
            print(f"发送命令失败: {e}")
            return False

    def send_multiple_commands(self, commands: List[str], delay: float = 0.1) -> int:
        """发送多个命令

        Args:
            commands: 命令列表
            delay: 命令间延迟（秒）

        Returns:
            成功发送的命令数量
        """
        success_count = 0

        for i, cmd in enumerate(commands):
            cmd = cmd.strip()
            if not cmd:  # 跳过空行
                continue

            print(f"\n命令 {i+1}/{len(commands)}: {cmd}")
            if self.send_hex_command(cmd):
                success_count += 1

            # 命令间延迟
            if delay > 0 and i < len(commands) - 1:
                time.sleep(delay)

        print(
            f"\n完成! 成功发送 {success_count}/{len([c for c in commands if c.strip()])} 个命令"
        )
        return success_count

    def calculate_checksum(self, rpm: int) -> int:
        """计算转速指令的校验和"""
        # 构造前6个字节：5aa52104 + 转速的小端序字节
        speed_bytes = rpm.to_bytes(2, "little")

        # 前6个字节
        byte0 = 0x5A
        byte1 = 0xA5
        byte2 = 0x21
        byte3 = 0x04
        byte4 = speed_bytes[0]  # 转速低字节
        byte5 = speed_bytes[1]  # 转速高字节

        # 校验字节 = (前6字节的和 + 1) & 0xFF
        checksum = (byte0 + byte1 + byte2 + byte3 + byte4 + byte5 + 1) & 0xFF
        return checksum

    def enter_realtime_speed_mode(self) -> bool:
        """进入实时转速更改模式"""
        print("进入实时转速更改模式...")
        return self.send_hex_command("5aa523022500000000000000000000000000000000000000")

    def set_fan_speed(self, rpm: int) -> bool:
        """设置风扇转速

        Args:
            rpm: 转速值 (建议范围: 1000-4000)
        """
        if not 0 <= rpm <= 65535:
            print(f"转速值超出范围: {rpm} (有效范围: 0-65535)")
            return False

        # 将转速转换为小端序字节
        speed_bytes = rpm.to_bytes(2, "little")

        # 计算校验和
        checksum = self.calculate_checksum(rpm)

        # 构造完整命令
        command = (
            f"5aa52104{speed_bytes.hex()}{checksum:02x}00000000000000000000000000000000"
        )

        print(f"设置风扇转速: {rpm} RPM")
        print(f"命令: {command}")

        return self.send_hex_command(command)

    def set_gear_position(self, gear: int, position: int) -> bool:
        """设置挡位

        Args:
            gear: 档位 (1-4)
            position: 档位内位置 (1-3)
        """
        gear_positions = {
            (1, 1): "5aa526050014054400000000000000000000000000000000",
            (1, 2): "5aa5260500a406d500000000000000000000000000000000",
            (1, 3): "5aa52605006c079e00000000000000000000000000000000",
            (2, 1): "5aa526050134086800000000000000000000000000000000",
            (2, 2): "5aa526050160099500000000000000000000000000000000",
            (2, 3): "5aa52605018c0ac200000000000000000000000000000000",
            (3, 1): "5aa5260502f00a2700000000000000000000000000000000",
            (3, 2): "5aa5260502b80bf000000000000000000000000000000000",
            (3, 3): "5aa5260502e40c1d00000000000000000000000000000000",
            (4, 1): "5aa5260503ac0de700000000000000000000000000000000",
            (4, 2): "5aa5260503740eb000000000000000000000000000000000",
            (4, 3): "5aa5260503a00fdd00000000000000000000000000000000",
        }

        if (gear, position) not in gear_positions:
            print(f"无效的挡位设置: {gear}档位{position}")
            print("有效范围: 1-4档，每档1-3个位置")
            return False

        command = gear_positions[(gear, position)]
        print(f"设置挡位: {gear}档位{position}")
        return self.send_hex_command(command)

    def set_gear_light(self, enabled: bool) -> bool:
        """设置挡位灯开关"""
        if enabled:
            command = "5aa54803014c000000000000000000000000000000000000"
            print("开启挡位灯")
        else:
            command = "5aa54803004b000000000000000000000000000000000000"
            print("关闭挡位灯")
        return self.send_hex_command(command)

    def set_power_on_start(self, enabled: bool) -> bool:
        """设置通电自启动"""
        if enabled:
            command = "5aa50c030110000000000000000000000000000000000000"
            print("开启通电自启动")
        else:
            command = "5aa50c030211000000000000000000000000000000000000"
            print("关闭通电自启动")
        return self.send_hex_command(command)

    def set_smart_start_stop(self, mode: str) -> bool:
        """设置智能启停

        Args:
            mode: 'off', 'immediate', 'delayed'
        """
        commands = {
            "off": "5aa50d030010000000000000000000000000000000000000",
            "immediate": "5aa50d030111000000000000000000000000000000000000",
            "delayed": "5aa50d030212000000000000000000000000000000000000",
        }

        if mode not in commands:
            print(f"无效的智能启停模式: {mode}")
            print("有效模式: 'off', 'immediate', 'delayed'")
            return False

        print(f"设置智能启停: {mode}")
        return self.send_hex_command(commands[mode])

    def set_brightness(self, percentage: int) -> bool:
        """设置灯光亮度

        Args:
            percentage: 亮度百分比 (0-100)
        """
        if percentage == 0:
            command = "5aa5470d1c00ff00000000000000006f0000000000000000"
            print("设置亮度: 0%")
        elif percentage == 100:
            command = "5aa543024500000000000000000000000000000000000000"
            print("设置亮度: 100%")
        else:
            print(f"当前仅支持0%和100%亮度设置")
            return False

        return self.send_hex_command(command)

    def disconnect(self):
        """断开连接"""
        if self.device:
            try:
                self.device.close()
            except:
                pass
            self.device = None
            print("设备已断开连接")


def test_bs2pro_with_commands():
    """测试 BS2PRO 设备并发送自定义命令"""
    controller = BS2PROHIDController()

    print("连接到 BS2PRO 设备...")
    print("=" * 50)

    # 直接使用已知的厂商ID和产品ID
    if not controller.connect(0x137D7, 0x1002):
        print("无法连接到 BS2PRO 设备")
        return False

    print("\n开始发送命令...")
    print("-" * 30)

    # 测试命令列表 - 每行一个命令
    test_commands = [
        "5aa54803014b0",
    ]

    # 发送多个命令
    controller.send_multiple_commands(test_commands, delay=0.2)

    # 测试读取输入
    print("\n监听输入数据...")
    print("  (按住手柄按键可能会看到数据...)")
    for i in range(3):
        response = controller.read_input_report(1000)
        if response and any(response):
            print(f"  输入 {i+1}: {response[:16].hex()}...")
            break
        else:
            print(f"  输入 {i+1}: 无数据")

    print("\n测试完成!")
    controller.disconnect()
    return True


def interactive_command_mode():
    """交互式命令模式"""
    controller = BS2PROHIDController()

    print("交互式命令模式")
    print("=" * 50)

    if not controller.connect(0x137D7, 0x1002):
        print("无法连接到 BS2PRO 设备")
        return

    print("\n使用说明:")
    print("  - 输入十六进制命令 (如: 5aa5410243)")
    print("  - 多行输入用回车分隔，空行结束")
    print("  - 输入 'quit' 或 'exit' 退出")
    print("  - 输入 'listen' 监听输入数据")
    print("  - 输入 'speed <rpm>' 设置转速 (如: speed 2000)")
    print("  - 输入 'gear <档位> <位置>' 设置挡位 (如: gear 2 3)")
    print("-" * 30)

    while True:
        try:
            print("\n请输入命令:")
            line = input().strip()

            if not line:
                continue

            if line.lower() in ["quit", "exit"]:
                break

            if line.lower() == "listen":
                print("监听模式 (5秒)...")
                for i in range(5):
                    response = controller.read_input_report(1000)
                    if response and any(response):
                        print(f"  输入: {response[:16].hex()}...")
                continue

            if line.lower().startswith("speed "):
                try:
                    rpm = int(line.split()[1])
                    controller.enter_realtime_speed_mode()
                    time.sleep(0.1)
                    controller.set_fan_speed(rpm)
                except (IndexError, ValueError):
                    print("用法: speed <rpm值>")
                continue

            if line.lower().startswith("gear "):
                try:
                    parts = line.split()
                    gear = int(parts[1])
                    position = int(parts[2])
                    controller.set_gear_position(gear, position)
                except (IndexError, ValueError):
                    print("用法: gear <档位> <位置>")
                continue

            # 普通十六进制命令
            controller.send_hex_command(line)

        except KeyboardInterrupt:
            print("\n\n用户中断，退出...")
            break
        except Exception as e:
            print(f"错误: {e}")

    controller.disconnect()


if __name__ == "__main__":
    print("选择模式:")
    print("1. 测试预设命令")
    print("2. 交互式命令模式")

    choice = input("请选择 (1/2): ").strip()

    if choice == "2":
        interactive_command_mode()
    else:
        test_bs2pro_with_commands()

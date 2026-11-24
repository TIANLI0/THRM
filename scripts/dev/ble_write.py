import asyncio
from winrt.windows.devices.bluetooth import BluetoothLEDevice, BluetoothCacheMode
from winrt.windows.devices.bluetooth.genericattributeprofile import (
    GattCharacteristicProperties,
)
from winrt.windows.storage.streams import DataWriter, Buffer
from winrt.windows.devices.enumeration import DeviceInformation

# 添加 HID 支持
try:
    import hid

    HID_AVAILABLE = True
    from hid_controller import BS2PROHIDController
except ImportError:
    HID_AVAILABLE = False
    print("⚠️ hidapi 未安装，HID 模式不可用。运行 'pip install hidapi' 安装。")


async def find_paired_ble_devices():
    """查找所有已配对的 BLE 设备"""
    try:
        # 使用设备枚举器查找所有 BLE 设备
        device_selector = BluetoothLEDevice.get_device_selector()
        devices = await DeviceInformation.find_all_async()

        print(f"找到 {len(devices)} 个 BLE 设备:")

        for device in devices:
            if device.name:  # 只显示有名称的设备
                print(f"设备名称: {device.name}")
                print(f"设备ID: {device.id}")
                print(f"是否已启用: {device.is_enabled}")
                print(f"是否已配对: {device.pairing.is_paired}")
                print("---")

                # 如果找到 BS2PRO 设备
                if "BS2PRO" in device.name.upper():
                    print(f"✓ 找到目标设备: {device.name}")
                    return device.id

        return None

    except Exception as e:
        print(f"枚举设备错误: {e}")
        return None


def analyze_characteristic_properties(properties):
    """分析特征值属性"""
    prop_list = []

    # 使用 GattCharacteristicProperties 枚举值
    if properties & GattCharacteristicProperties.READ:
        prop_list.append("READ")
    if properties & GattCharacteristicProperties.WRITE:
        prop_list.append("WRITE")
    if properties & GattCharacteristicProperties.WRITE_WITHOUT_RESPONSE:
        prop_list.append("WRITE_NO_RESPONSE")
    if properties & GattCharacteristicProperties.NOTIFY:
        prop_list.append("NOTIFY")
    if properties & GattCharacteristicProperties.INDICATE:
        prop_list.append("INDICATE")
    if properties & GattCharacteristicProperties.BROADCAST:
        prop_list.append("BROADCAST")
    if properties & GattCharacteristicProperties.EXTENDED_PROPERTIES:
        prop_list.append("EXTENDED")
    if properties & GattCharacteristicProperties.AUTHENTICATED_SIGNED_WRITES:
        prop_list.append("AUTH_SIGNED_WRITES")

    return prop_list


def get_service_description(uuid_str):
    """获取标准服务描述"""
    standard_services = {}
    return standard_services.get(uuid_str.lower(), "Unknown Service")


def format_gatt_status(code: int) -> str:
    """格式化 GATT 状态码"""
    mapping = {
        0: "Success",
        1: "Unreachable",
        2: "ProtocolError",
        3: "AccessDenied",
    }
    return mapping.get(int(code), f"Unknown({code})")


async def discover_services_and_characteristics(device):
    """发现设备的所有服务和特征值"""
    try:
        print(f"\n🔍 正在分析设备: {device.name}")
        print(f"设备地址: {hex(device.bluetooth_address)}")
        print("=" * 60)

        # 获取GATT服务
        gatt_result = await device.get_gatt_services_async()

        if gatt_result.status != 0:
            print(f"获取服务失败，状态码: {gatt_result.status}")
            return

        services = gatt_result.services
        print(f"📋 找到 {len(services)} 个服务:\n")

        for i, service in enumerate(services, 1):
            service_uuid = str(service.uuid)
            service_desc = get_service_description(service_uuid)

            print(f"🔧 服务 {i}: {service_desc}")
            print(f"   UUID: {service_uuid}")

            # 获取特征值（尝试使用未缓存模式）
            try:
                char_result = await service.get_characteristics_async(
                    BluetoothCacheMode.UNCACHED
                )
            except TypeError:
                char_result = await service.get_characteristics_async()

            if char_result.status == 0:
                characteristics = char_result.characteristics
                print(f"   📊 特征值数量: {len(characteristics)}")

                for j, char in enumerate(characteristics, 1):
                    char_uuid = str(char.uuid)
                    properties = analyze_characteristic_properties(
                        char.characteristic_properties
                    )

                    print(f"      📌 特征值 {j}:")
                    print(f"         UUID: {char_uuid}")
                    print(f"         属性: {', '.join(properties)}")

                    # 标明读写能力
                    capabilities = []
                    if "READ" in properties:
                        capabilities.append("✅ 可读")
                    if "WRITE" in properties or "WRITE_NO_RESPONSE" in properties:
                        capabilities.append("✏️ 可写")
                    if "NOTIFY" in properties:
                        capabilities.append("🔔 可通知")
                    if "INDICATE" in properties:
                        capabilities.append("📢 可指示")

                    if capabilities:
                        print(f"         功能: {' | '.join(capabilities)}")

                    # 如果支持读取，尝试读取描述符
                    try:
                        descriptors_result = await char.get_descriptors_async()
                        if (
                            descriptors_result.status == 0
                            and len(descriptors_result.descriptors) > 0
                        ):
                            print(
                                f"         描述符: {len(descriptors_result.descriptors)} 个"
                            )
                    except:  # noqa: E722
                        pass

                    print()
            else:
                status_desc = format_gatt_status(char_result.status)
                print(f"   ❌ 无法获取特征值，状态: {status_desc}")

                # 如果是 HID 服务且访问被拒绝，提示使用 HID 模式
                if (
                    service_uuid == "00001812-0000-1000-8000-00805f9b34fb"
                    and char_result.status == 3
                    and HID_AVAILABLE
                ):
                    print(f"   💡 这是 HID 服务，建议使用 HID 模式访问")

            print("-" * 50)

    except Exception as e:
        print(f"发现服务错误: {e}")


async def connect_to_paired_device():
    """连接到已配对的 BS2PRO 设备"""
    try:
        # 方法1: 通过 MAC 地址连接 (修正MAC地址)
        mac_address = 0xE566E510CE04  # 移除末尾的5

        try:
            print("🔗 尝试通过MAC地址连接...")
            ble_device = await BluetoothLEDevice.from_bluetooth_address_async(
                mac_address
            )

            if ble_device is not None:
                print(f"✅ 通过MAC地址连接成功: {ble_device.name}")
                await discover_services_and_characteristics(ble_device)
                return ble_device

        except Exception as mac_error:
            print(f"❌ MAC地址连接失败: {mac_error}")

        # 方法2: 通过设备枚举查找
        print("🔍 尝试通过设备枚举查找...")
        device_id = await find_paired_ble_devices()

        if device_id:
            ble_device = await BluetoothLEDevice.from_id_async(device_id)

            if ble_device is None:
                print("❌ 无法创建设备对象")
                return None

            print(f"✅ 连接到设备: {ble_device.name}")
            await discover_services_and_characteristics(ble_device)
            return ble_device
        else:
            print("❌ 未找到 BS2PRO 设备")
            return None

    except Exception as e:
        print(f"❌ 连接错误: {e}")
        return None


# 使用示例
async def main():
    """主函数"""
    print("🚀 开始搜索并分析 BS2PRO 设备...")
    print("\n选择连接模式:")
    print("1. BLE GATT 模式 (默认)")
    if HID_AVAILABLE:
        print("2. HID 模式")

    choice = input("\n请选择模式 (1/2): ").strip()

    if choice == "2" and HID_AVAILABLE:
        # 使用 HID 模式
        print("\n🔧 使用 HID 模式连接...")
        controller = BS2PROHIDController()

        if controller.connect():
            print("\n🧪 执行 HID 通信测试...")
            # 这里可以添加具体的 HID 命令测试
            controller.disconnect()
        return

    # 默认使用 BLE GATT 模式
    print("\n🔧 使用 BLE GATT 模式连接...")
    device = await connect_to_paired_device()

    if device:
        print("\n✅ 设备分析完成!")
        print(f"设备名称: {device.name}")
        print(f"设备地址: {hex(device.bluetooth_address)}")
        print("\n💡 提示: 查看上方输出找到可写的特征值UUID用于发送数据")

        if HID_AVAILABLE:
            print("💡 如需访问 HID 服务，请重新运行并选择 HID 模式")
    else:
        print("\n❌ 未能连接到设备")
        print("\n🔧 故障排除建议:")
        print("1. 确保设备已在Windows设置中配对")
        print("2. 确保设备处于开启状态")
        print("3. 尝试在蓝牙设置中断开并重新连接设备")
        if HID_AVAILABLE:
            print("4. 尝试使用 HID 模式连接")


if __name__ == "__main__":
    asyncio.run(main())

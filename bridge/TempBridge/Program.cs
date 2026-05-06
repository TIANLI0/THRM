using System;
using System.IO;
using System.IO.Pipes;
using System.Linq;
using System.ServiceProcess;
using System.Threading;
using Newtonsoft.Json;
using LibreHardwareMonitor.Hardware;
using LibreHardwareMonitor.PawnIo;

namespace TempBridge
{
    public class TemperatureData
    {
        public int CpuTemp { get; set; }
        public int GpuTemp { get; set; }
        public int MaxTemp { get; set; }
        public int ControlTemp { get; set; }
        public string ControlSource { get; set; }
        public string SelectedGpuDevice { get; set; }
        public string CpuModel { get; set; }
        public string GpuModel { get; set; }
        public TemperatureSensor[] CpuSensors { get; set; }
        public TemperatureSensor[] GpuSensors { get; set; }
        public TemperatureGpuDevice[] GpuDevices { get; set; }
        public long UpdateTime { get; set; }
        public bool Success { get; set; }
        public string Error { get; set; }

        public TemperatureData()
        {
            ControlSource = "max";
            SelectedGpuDevice = "auto";
            CpuModel = string.Empty;
            GpuModel = string.Empty;
            CpuSensors = Array.Empty<TemperatureSensor>();
            GpuSensors = Array.Empty<TemperatureSensor>();
            GpuDevices = Array.Empty<TemperatureGpuDevice>();
            Error = string.Empty;
        }
    }

    public class TemperatureSensor
    {
        public string Key { get; set; }
        public string Name { get; set; }
        public int Value { get; set; }

        public TemperatureSensor()
        {
            Key = string.Empty;
            Name = string.Empty;
        }
    }

    public class TemperatureGpuDevice
    {
        public string Key { get; set; }
        public string Name { get; set; }
        public string Vendor { get; set; }
        public TemperatureSensor[] Sensors { get; set; }

        public TemperatureGpuDevice()
        {
            Key = "auto";
            Name = string.Empty;
            Vendor = string.Empty;
            Sensors = Array.Empty<TemperatureSensor>();
        }
    }

    public class TemperatureSelection
    {
        public string TempSource { get; set; }
        public string GpuDevice { get; set; }
        public string CpuSensor { get; set; }
        public string GpuSensor { get; set; }

        public TemperatureSelection()
        {
            TempSource = "max";
            GpuDevice = "auto";
            CpuSensor = "auto";
            GpuSensor = "auto";
        }
    }

    public class GpuCandidate
    {
        public string Key { get; set; }
        public string Model { get; set; }
        public string Vendor { get; set; }
        public HardwareType HardwareType { get; set; }
        public System.Collections.Generic.List<TemperatureSensor> Sensors { get; set; }

        public GpuCandidate()
        {
            Key = "auto";
            Model = string.Empty;
            Vendor = string.Empty;
            Sensors = new System.Collections.Generic.List<TemperatureSensor>();
        }
    }

    public class UpdateVisitor : IVisitor
    {
        public void VisitComputer(IComputer computer)
        {
            computer.Traverse(this);
        }

        public void VisitHardware(IHardware hardware)
        {
            hardware.Update();
            foreach (IHardware subHardware in hardware.SubHardware)
                subHardware.Accept(this);
        }

        public void VisitSensor(ISensor sensor) { }
        public void VisitParameter(IParameter parameter) { }
    }

    public class Command
    {
        public string Type { get; set; }
        public string Data { get; set; }
    }

    public class Response
    {
        public bool Success { get; set; }
        public string Error { get; set; }
        public TemperatureData Data { get; set; }
    }

    class Program
    {
        private const string PipeName = "BS2PRO_TempBridge";
        private const string MutexName = @"Global\BS2PRO_TempBridge_Singleton";
        private const int MaxInitRetries = 3;
        private const int InitRetryDelayMs = 2000;
        private const int ConsecutiveFailuresBeforeReinit = 5;
        private static Computer computer;
        private static bool running = true;
        private static readonly object lockObject = new object();
        private static Mutex singleInstanceMutex;
        private static int consecutiveFailures = 0;

        static void Main(string[] args)
        {
            try
            {
                if (ShouldRunDiagnosticMode(args))
                {
                    RunConsoleDiagnostics();
                    return;
                }

                // 初始化硬件监控
                using (var instanceHandle = AcquirePipeInstance())
                {
                    if (instanceHandle == null)
                    {
                        Console.WriteLine($"PIPE:{PipeName}|ATTACH");
                        Console.Out.Flush();
                        return;
                    }

                    InitializeHardwareMonitor();

                    // 输出管道名称，让主程序知道如何连接
                    Console.WriteLine($"PIPE:{PipeName}|OWNER");
                    Console.Out.Flush();

                    // 启动管道服务器
                    StartPipeServer();
                }
            }
            catch (Exception ex)
            {
                if (ShouldRunDiagnosticMode(args))
                {
                    Console.Error.WriteLine("TempBridge 启动失败");
                    Console.Error.WriteLine($"错误: {ex.Message}");
                }
                else
                {
                    Console.WriteLine($"ERROR:{ex.Message}");
                }
                Environment.Exit(1);
            }
            finally
            {
                computer?.Close();
                if (singleInstanceMutex != null)
                {
                    singleInstanceMutex.Dispose();
                    singleInstanceMutex = null;
                }
            }
        }

        static IDisposable AcquirePipeInstance()
        {
            bool createdNew;
            singleInstanceMutex = new Mutex(false, MutexName, out createdNew);

            bool acquired = false;
            try
            {
                acquired = singleInstanceMutex.WaitOne(0, false);
            }
            catch (AbandonedMutexException)
            {
                acquired = true;
            }

            if (!acquired)
            {
                return null;
            }

            return new MutexHandle(singleInstanceMutex);
        }

        static bool ShouldRunDiagnosticMode(string[] args)
        {
            if (HasArg(args, "--pipe"))
            {
                return false;
            }

            if (HasArg(args, "--diag") || HasArg(args, "--diagnose"))
            {
                return true;
            }

            return Environment.UserInteractive && !Console.IsOutputRedirected;
        }

        static bool HasArg(string[] args, string expected)
        {
            if (args == null || args.Length == 0)
            {
                return false;
            }

            return args.Any(arg => string.Equals(arg, expected, StringComparison.OrdinalIgnoreCase));
        }

        static void RunConsoleDiagnostics()
        {
            Console.WriteLine("TempBridge 诊断模式");
            Console.WriteLine($"时间: {DateTime.Now:yyyy-MM-dd HH:mm:ss}");
            Console.WriteLine();

            InitializeHardwareMonitor();

            TemperatureData data = GetTemperatureData(new TemperatureSelection());
            PrintTemperatureSummary(data);
            Console.WriteLine();
            PrintHardwareSnapshot();

            if (!data.Success)
            {
                Environment.Exit(1);
            }
        }

        static void PrintTemperatureSummary(TemperatureData data)
        {
            Console.WriteLine("温度结果");
            Console.WriteLine($"CPU: {FormatTemperature(data.CpuTemp)}");
            Console.WriteLine($"GPU: {FormatTemperature(data.GpuTemp)}");
            Console.WriteLine($"MAX: {FormatTemperature(data.MaxTemp)}");
            Console.WriteLine($"Success: {data.Success}");

            if (!string.IsNullOrEmpty(data.Error))
            {
                Console.WriteLine($"Error: {data.Error}");
            }
        }

        static string FormatTemperature(int value)
        {
            return value > 0 ? value + "°C" : "N/A";
        }

        static void PrintHardwareSnapshot()
        {
            Console.WriteLine("温度传感器快照");

            bool foundAny = false;
            foreach (IHardware hardware in computer.Hardware)
            {
                foundAny |= PrintHardwareSnapshotRecursive(hardware, 0);
            }

            if (!foundAny)
            {
                Console.WriteLine("- 未发现可用的温度传感器");
            }
        }

        static bool PrintHardwareSnapshotRecursive(IHardware hardware, int indentLevel)
        {
            bool wroteLine = false;
            string indent = new string(' ', indentLevel * 2);

            foreach (ISensor sensor in hardware.Sensors)
            {
                if (sensor.SensorType != SensorType.Temperature)
                {
                    continue;
                }

                string valueText = sensor.Value.HasValue
                    ? sensor.Value.Value.ToString("F1") + "°C"
                    : "N/A";
                Console.WriteLine(
                    string.Format(
                        "{0}- [{1}] {2} / {3}: {4}",
                        indent,
                        hardware.HardwareType,
                        hardware.Name,
                        sensor.Name,
                        valueText
                    )
                );
                wroteLine = true;
            }

            foreach (IHardware subHardware in hardware.SubHardware)
            {
                if (PrintHardwareSnapshotRecursive(subHardware, indentLevel + 1))
                {
                    wroteLine = true;
                }
            }

            return wroteLine;
        }

        static void InitializeHardwareMonitor()
        {
            EnsurePawnIoReady();

            Exception lastException = null;
            for (int attempt = 1; attempt <= MaxInitRetries; attempt++)
            {
                try
                {
                    if (computer != null)
                    {
                        try { computer.Close(); } catch { }
                        computer = null;
                    }

                    computer = new Computer
                    {
                        IsCpuEnabled = true,
                        IsGpuEnabled = true,
                        IsMemoryEnabled = false,
                        IsMotherboardEnabled = false,
                        IsControllerEnabled = false,
                        IsNetworkEnabled = false,
                        IsStorageEnabled = false
                    };

                    computer.Open();
                    computer.Accept(new UpdateVisitor());

                    // Verify we can actually read at least one temperature
                    if (HasAnyTemperatureSensor(computer))
                    {
                        consecutiveFailures = 0;
                        return;
                    }

                    // No sensors found - PawnIO may not be fully ready
                    if (attempt < MaxInitRetries)
                    {
                        computer.Close();
                        computer = null;
                        Thread.Sleep(InitRetryDelayMs);
                    }
                }
                catch (Exception ex)
                {
                    lastException = ex;
                    if (attempt < MaxInitRetries)
                    {
                        try { computer?.Close(); } catch { }
                        computer = null;
                        Thread.Sleep(InitRetryDelayMs);
                    }
                }
            }

            // If we get here, all retries exhausted but computer may still be open
            // (just without working sensors). Keep it - it might recover on next update.
            if (computer == null)
            {
                string msg = lastException != null
                    ? lastException.Message
                    : "初始化硬件监控失败，PawnIO 可能被其他程序占用";
                throw new InvalidOperationException(msg, lastException);
            }
        }

        static bool HasAnyTemperatureSensor(Computer comp)
        {
            foreach (IHardware hardware in comp.Hardware)
            {
                if (HasTemperatureSensorRecursive(hardware))
                    return true;
            }
            return false;
        }

        static bool HasTemperatureSensorRecursive(IHardware hardware)
        {
            foreach (ISensor sensor in hardware.Sensors)
            {
                if (sensor.SensorType == SensorType.Temperature && sensor.Value.HasValue && sensor.Value.Value > 0)
                    return true;
            }
            foreach (IHardware sub in hardware.SubHardware)
            {
                if (HasTemperatureSensorRecursive(sub))
                    return true;
            }
            return false;
        }

        static void EnsurePawnIoReady()
        {
            if (!PawnIo.IsInstalled)
            {
                throw new InvalidOperationException(
                    "检测到 LibreHardwareMonitor 需要 PawnIO 驱动，但系统未安装。" +
                    "请先安装 PawnIO（可从 LibreHardwareMonitor 发布包中的 PawnIO_setup.exe 安装），" +
                    "安装完成后重启程序。"
                );
            }

            // Check PawnIO driver service is running; attempt start if stopped
            try
            {
                using (var sc = new ServiceController("PawnIO"))
                {
                    if (sc.Status != ServiceControllerStatus.Running)
                    {
                        sc.Start();
                        sc.WaitForStatus(ServiceControllerStatus.Running, TimeSpan.FromSeconds(10));
                    }
                }
            }
            catch (InvalidOperationException)
            {
                // Service not found - PawnIO may use a different service name, continue
            }
            catch (System.ServiceProcess.TimeoutException)
            {
                throw new InvalidOperationException(
                    "PawnIO 驱动服务未能在 10 秒内启动，请检查驱动安装状态。"
                );
            }
        }

        static void ReinitializeHardwareMonitor()
        {
            lock (lockObject)
            {
                try
                {
                    computer?.Close();
                }
                catch { }
                computer = null;

                // Try driver restart (best-effort, kernel drivers often reject stop)
                RestartPawnIoDriver();

                // Wait a moment for driver/device objects to settle
                Thread.Sleep(500);

                InitializeHardwareMonitor();
            }
        }

        /// <summary>
        /// Best-effort PawnIO driver restart. Kernel drivers typically reject
        /// SERVICE_CONTROL_STOP (error 1052), so this method tries stop+start
        /// first, falls back to just ensuring the service is running.
        /// The real recovery comes from closing and reopening the Computer object
        /// to get a fresh device handle — not from restarting the driver itself.
        /// </summary>
        static string RestartPawnIoDriver()
        {
            try
            {
                using (var sc = new ServiceController("PawnIO"))
                {
                    // Attempt stop+start (may fail for kernel drivers)
                    if (sc.Status == ServiceControllerStatus.Running)
                    {
                        try
                        {
                            sc.Stop();
                            sc.WaitForStatus(ServiceControllerStatus.Stopped, TimeSpan.FromSeconds(5));
                        }
                        catch
                        {
                            // Kernel driver rejected stop — this is normal.
                            // Driver is still running, just re-acquire a handle.
                            return null;
                        }
                    }

                    // If stopped (either by us or was already stopped), start it
                    sc.Refresh();
                    if (sc.Status != ServiceControllerStatus.Running)
                    {
                        sc.Start();
                        sc.WaitForStatus(ServiceControllerStatus.Running, TimeSpan.FromSeconds(10));
                        Thread.Sleep(500);
                    }
                }

                return null;
            }
            catch (InvalidOperationException)
            {
                // Service not found — driver may use a different registration method
                return null;
            }
            catch (Exception)
            {
                // Any other error — continue with handle refresh anyway
                return null;
            }
        }

        static void StartPipeServer()
        {
            while (running)
            {
                try
                {
                    using (var pipeServer = new NamedPipeServerStream(PipeName, PipeDirection.InOut))
                    {
                        // 等待客户端连接
                        pipeServer.WaitForConnection();

                        using (var reader = new StreamReader(pipeServer))
                        using (var writer = new StreamWriter(pipeServer))
                        {
                            while (pipeServer.IsConnected && running)
                            {
                                try
                                {
                                    string commandJson = reader.ReadLine();
                                    if (string.IsNullOrEmpty(commandJson))
                                        break;

                                    var command = JsonConvert.DeserializeObject<Command>(commandJson);
                                    var response = ProcessCommand(command);

                                    string responseJson = JsonConvert.SerializeObject(response);
                                    writer.WriteLine(responseJson);
                                    writer.Flush();

                                    if (command.Type == "Exit")
                                    {
                                        running = false;
                                        break;
                                    }
                                }
                                catch (Exception ex)
                                {
                                    var errorResponse = new Response
                                    {
                                        Success = false,
                                        Error = ex.Message
                                    };
                                    string errorJson = JsonConvert.SerializeObject(errorResponse);
                                    writer.WriteLine(errorJson);
                                    writer.Flush();
                                    break;
                                }
                            }
                        }
                    }
                }
                catch (Exception ex)
                {
                    if (running)
                    {
                        Console.WriteLine($"管道错误: {ex.Message}");
                        Thread.Sleep(1000); // 等待一秒后重试
                    }
                }
            }
        }

        static Response ProcessCommand(Command command)
        {
            try
            {
                switch (command.Type)
                {
                    case "GetTemperature":
                        var selection = ParseTemperatureSelection(command.Data);
                        var data = GetTemperatureData(selection);
                        return new Response
                        {
                            Success = data.Success,
                            Error = data.Success ? string.Empty : data.Error,
                            Data = data
                        };

                    case "Ping":
                        return new Response
                        {
                            Success = true,
                            Data = new TemperatureData { Success = true }
                        };

                    case "RestartPawnIO":
                        return HandleRestartPawnIO();

                    case "Exit":
                        return new Response
                        {
                            Success = true
                        };

                    default:
                        return new Response
                        {
                            Success = false,
                            Error = "未知命令类型"
                        };
                }
            }
            catch (Exception ex)
            {
                return new Response
                {
                    Success = false,
                    Error = ex.Message
                };
            }
        }

        static Response HandleRestartPawnIO()
        {
            lock (lockObject)
            {
                // 1. Close existing Computer to release PawnIO handle
                try
                {
                    computer?.Close();
                }
                catch { }
                computer = null;

                // 2. Best-effort driver restart (kernel drivers usually reject stop, that's OK)
                RestartPawnIoDriver();

                // 3. Wait for device to settle after handle release
                Thread.Sleep(500);

                // 4. Reinitialize hardware monitor with fresh PawnIO handle
                try
                {
                    InitializeHardwareMonitor();
                    consecutiveFailures = 0;

                    // 5. Do a test read to confirm it works
                    var testData = GetTemperatureDataUnsafe(new TemperatureSelection());
                    return new Response
                    {
                        Success = testData.Success,
                        Error = testData.Success ? string.Empty : testData.Error,
                        Data = testData
                    };
                }
                catch (Exception ex)
                {
                    return new Response
                    {
                        Success = false,
                        Error = string.Format("重新初始化失败: {0}", ex.Message)
                    };
                }
            }
        }

        /// <summary>
        /// GetTemperatureData without acquiring lockObject (caller must hold the lock).
        /// </summary>
        static TemperatureSelection ParseTemperatureSelection(string raw)
        {
            if (string.IsNullOrWhiteSpace(raw))
            {
                return new TemperatureSelection();
            }

            try
            {
                return NormalizeTemperatureSelection(
                    JsonConvert.DeserializeObject<TemperatureSelection>(raw) ?? new TemperatureSelection()
                );
            }
            catch
            {
                return new TemperatureSelection();
            }
        }

        static TemperatureSelection NormalizeTemperatureSelection(TemperatureSelection selection)
        {
            if (selection == null)
            {
                return new TemperatureSelection();
            }

            selection.TempSource = NormalizeTempSource(selection.TempSource);
            selection.GpuDevice = NormalizeDeviceSelection(selection.GpuDevice);
            selection.CpuSensor = NormalizeSensorSelection(selection.CpuSensor);
            selection.GpuSensor = NormalizeSensorSelection(selection.GpuSensor);
            return selection;
        }

        static string NormalizeTempSource(string source)
        {
            if (string.Equals(source, "cpu", StringComparison.OrdinalIgnoreCase))
            {
                return "cpu";
            }
            if (string.Equals(source, "gpu", StringComparison.OrdinalIgnoreCase))
            {
                return "gpu";
            }
            return "max";
        }

        static string NormalizeDeviceSelection(string deviceKey)
        {
            return string.IsNullOrWhiteSpace(deviceKey) ? "auto" : deviceKey;
        }

        static string NormalizeSensorSelection(string sensorKey)
        {
            return string.IsNullOrWhiteSpace(sensorKey) ? "auto" : sensorKey;
        }

        static TemperatureData GetTemperatureDataUnsafe(TemperatureSelection selection)
        {
            selection = NormalizeTemperatureSelection(selection);
            var result = new TemperatureData
            {
                UpdateTime = DateTimeOffset.UtcNow.ToUnixTimeSeconds(),
                ControlSource = selection.TempSource
            };

            try
            {
                computer.Accept(new UpdateVisitor());

                string cpuModel = string.Empty;
                string gpuModel = string.Empty;
                var cpuSensors = new System.Collections.Generic.List<TemperatureSensor>();
                var gpuCandidates = new System.Collections.Generic.List<GpuCandidate>();
                int gpuIndex = 0;

                foreach (IHardware hardware in computer.Hardware)
                {
                    if (hardware.HardwareType == HardwareType.Cpu)
                    {
                        	if (cpuSensors.Count == 0)
                        	{
                        		cpuModel = hardware.Name ?? string.Empty;
                        		CollectTemperatureSensors(hardware, "cpu", hardware.Name ?? string.Empty, string.Empty, cpuSensors);
                        	}
                    }
                    else if (hardware.HardwareType == HardwareType.GpuNvidia || 
                             hardware.HardwareType == HardwareType.GpuAmd ||
                             hardware.HardwareType == HardwareType.GpuIntel)
                    {
                        	var sensors = new System.Collections.Generic.List<TemperatureSensor>();
                        	CollectTemperatureSensors(hardware, "gpu", hardware.Name ?? string.Empty, string.Empty, sensors);
                        	gpuCandidates.Add(new GpuCandidate
                        	{
                        		Key = BuildGpuDeviceKey(hardware, gpuIndex),
                        		Model = hardware.Name ?? string.Empty,
                        		Vendor = GetGpuVendor(hardware.HardwareType),
                        		HardwareType = hardware.HardwareType,
                        		Sensors = sensors,
                        	});
                        	gpuIndex++;
                    }
                }

                    var selectedGpu = SelectGpuCandidate(gpuCandidates, selection.GpuDevice, selection.GpuSensor);
                    var gpuSensors = selectedGpu != null ? selectedGpu.Sensors : new System.Collections.Generic.List<TemperatureSensor>();
                    gpuModel = selectedGpu != null ? selectedGpu.Model : string.Empty;

	                int cpuTemp = SelectTemperature(cpuSensors, selection.CpuSensor, new[] { "Average", "Package", "Tctl", "Tdie", "Core" });
	                int gpuTemp = SelectTemperature(gpuSensors, selection.GpuSensor, new[] { "Average", "GPU Core", "Core", "Edge", "Junction", "Hot Spot", "Temperature" });

                result.CpuTemp = cpuTemp;
                result.GpuTemp = gpuTemp;
                result.MaxTemp = Math.Max(cpuTemp, gpuTemp);
	                result.ControlTemp = ResolveControlTemp(cpuTemp, gpuTemp, selection.TempSource);
                    result.SelectedGpuDevice = selectedGpu != null ? selectedGpu.Key : selection.GpuDevice;
	                result.CpuModel = cpuModel;
	                result.GpuModel = gpuModel;
	                result.CpuSensors = cpuSensors.ToArray();
	                result.GpuSensors = gpuSensors.ToArray();
                    result.GpuDevices = gpuCandidates.Select(candidate => new TemperatureGpuDevice
                    {
                        Key = candidate.Key,
                        Name = candidate.Model,
                        Vendor = candidate.Vendor,
                        Sensors = candidate.Sensors != null ? candidate.Sensors.ToArray() : Array.Empty<TemperatureSensor>()
                    }).ToArray();
                if (cpuTemp == 0 && gpuTemp == 0)
                {
                    result.Success = false;
                    result.Error = "未读取到有效的 CPU/GPU 温度";
                }
                else
                {
                    result.Success = true;
                    result.Error = string.Empty;
                }
            }
            catch (Exception ex)
            {
                result.Success = false;
                result.Error = ex.Message;
            }

            return result;
        }

        static TemperatureData GetTemperatureData(TemperatureSelection selection)
        {
            lock (lockObject)
            {
	                var result = GetTemperatureDataUnsafe(selection);

                if (!result.Success || (result.CpuTemp == 0 && result.GpuTemp == 0))
                {
                    consecutiveFailures++;

                    // Auto-reinitialize after consecutive failures (restart PawnIO driver + reinit)
                    if (consecutiveFailures >= ConsecutiveFailuresBeforeReinit)
                    {
                        consecutiveFailures = 0;
                        result.Error = "连续读取失败，正在尝试重启 PawnIO 驱动并重新初始化...";

                        ThreadPool.QueueUserWorkItem(_ =>
                        {
                            try { ReinitializeHardwareMonitor(); }
                            catch { }
                        });
                    }
                    else if (string.IsNullOrEmpty(result.Error))
                    {
                        result.Error = string.Format(
                            "未读取到有效的 CPU/GPU 温度（连续失败 {0}/{1}，达到阈值后将自动重启 PawnIO 驱动）",
                            consecutiveFailures, ConsecutiveFailuresBeforeReinit);
                    }
                }
                else
                {
                    consecutiveFailures = 0;
                }

                return result;
            }
        }

            static void CollectTemperatureSensors(IHardware hardware, string devicePrefix, string keyPath, string displayPath, System.Collections.Generic.List<TemperatureSensor> sensors)
        {
                foreach (ISensor sensor in hardware.Sensors)
                {
                    if (sensor.SensorType != SensorType.Temperature || !sensor.Value.HasValue)
                    {
                        continue;
                    }

                    int temp = (int)Math.Round(sensor.Value.Value);
                    if (temp <= 0 || temp >= 150)
                    {
                        continue;
                    }

                    string sensorPath = string.IsNullOrEmpty(keyPath) ? sensor.Name : keyPath + "/" + sensor.Name;
                    sensors.Add(new TemperatureSensor
                    {
                        Key = devicePrefix + "/" + sensorPath,
                        Name = string.IsNullOrEmpty(displayPath) ? sensor.Name : displayPath + " / " + sensor.Name,
                        Value = temp,
                    });
                }

                foreach (IHardware subHardware in hardware.SubHardware)
                {
                    string subKeyPath = string.IsNullOrEmpty(keyPath)
                        ? (subHardware.Name ?? string.Empty)
                        : keyPath + "/" + (subHardware.Name ?? string.Empty);
                    string subDisplayPath = string.IsNullOrEmpty(displayPath)
                        ? (subHardware.Name ?? string.Empty)
                        : displayPath + " / " + (subHardware.Name ?? string.Empty);
                    CollectTemperatureSensors(subHardware, devicePrefix, subKeyPath, subDisplayPath, sensors);
                }
        }

        static int SelectTemperature(System.Collections.Generic.IReadOnlyList<TemperatureSensor> sensors, string selectedKey, string[] preferredSensorNames)
        {
                if (sensors == null || sensors.Count == 0)
            {
                    return 0;
            }

                if (!string.Equals(selectedKey, "auto", StringComparison.OrdinalIgnoreCase))
                {
                    foreach (var sensor in sensors)
                    {
                        if (string.Equals(sensor.Key, selectedKey, StringComparison.OrdinalIgnoreCase))
                        {
                            return sensor.Value;
                        }
                    }
                }

                foreach (var sensor in sensors)
                {
                    if (ContainsAnyKeyword(sensor.Name, preferredSensorNames))
                    {
                        return sensor.Value;
                    }
                }

                return sensors[0].Value;
            }

        static GpuCandidate SelectGpuCandidate(System.Collections.Generic.IReadOnlyList<GpuCandidate> candidates, string selectedDeviceKey, string selectedSensorKey)
        {
            if (candidates == null || candidates.Count == 0)
            {
                return null;
            }

                if (!string.Equals(selectedDeviceKey, "auto", StringComparison.OrdinalIgnoreCase))
                {
                    foreach (var candidate in candidates)
                    {
                        if (candidate != null && string.Equals(candidate.Key, selectedDeviceKey, StringComparison.OrdinalIgnoreCase))
                        {
                            return candidate;
                        }
                    }
                }

                if (!string.Equals(selectedSensorKey, "auto", StringComparison.OrdinalIgnoreCase))
            {
                foreach (var candidate in candidates)
                {
                    if (candidate == null || candidate.Sensors == null)
                    {
                        continue;
                    }

                    foreach (var sensor in candidate.Sensors)
                    {
                            if (string.Equals(sensor.Key, selectedSensorKey, StringComparison.OrdinalIgnoreCase))
                        {
                            return candidate;
                        }
                    }
                }
            }

            GpuCandidate bestWithSensors = null;
            GpuCandidate bestFallback = null;

            foreach (var candidate in candidates)
            {
                if (candidate == null)
                {
                    continue;
                }

                if (bestFallback == null || CompareGpuCandidate(candidate, bestFallback) > 0)
                {
                    bestFallback = candidate;
                }

                if (candidate.Sensors != null && candidate.Sensors.Count > 0)
                {
                    if (bestWithSensors == null || CompareGpuCandidate(candidate, bestWithSensors) > 0)
                    {
                        bestWithSensors = candidate;
                    }
                }
            }

            return bestWithSensors ?? bestFallback;
        }

            static string BuildGpuDeviceKey(IHardware hardware, int index)
            {
                string vendor = GetGpuVendor(hardware.HardwareType);
                string name = hardware != null && !string.IsNullOrWhiteSpace(hardware.Name) ? hardware.Name.Trim() : "gpu";
                return string.Format("{0}:{1}:{2}", vendor, index, name);
            }

            static string GetGpuVendor(HardwareType hardwareType)
            {
                switch (hardwareType)
                {
                    case HardwareType.GpuNvidia:
                        return "nvidia";
                    case HardwareType.GpuAmd:
                        return "amd";
                    case HardwareType.GpuIntel:
                        return "intel";
                    default:
                        return "gpu";
                }
            }

        static int CompareGpuCandidate(GpuCandidate left, GpuCandidate right)
        {
            int leftPriority = GetGpuPriority(left);
            int rightPriority = GetGpuPriority(right);
            if (leftPriority != rightPriority)
            {
                return leftPriority - rightPriority;
            }

            int leftSensorCount = left != null && left.Sensors != null ? left.Sensors.Count : 0;
            int rightSensorCount = right != null && right.Sensors != null ? right.Sensors.Count : 0;
            return leftSensorCount - rightSensorCount;
        }

        static int GetGpuPriority(GpuCandidate candidate)
        {
            if (candidate == null)
            {
                return 0;
            }

            switch (candidate.HardwareType)
            {
                case HardwareType.GpuNvidia:
                    return 300;
                case HardwareType.GpuAmd:
                    return 200;
                case HardwareType.GpuIntel:
                    return 100;
                default:
                    return 0;
            }
        }

            static int ResolveControlTemp(int cpuTemp, int gpuTemp, string source)
            {
                switch (NormalizeTempSource(source))
                {
                    case "cpu":
                        return cpuTemp;
                    case "gpu":
                        return gpuTemp;
                    default:
                        return Math.Max(cpuTemp, gpuTemp);
                }
        }

        static bool ContainsAnyKeyword(string source, string[] keywords)
        {
            if (string.IsNullOrEmpty(source) || keywords == null)
            {
                return false;
            }

            foreach (string keyword in keywords)
            {
                if (!string.IsNullOrEmpty(keyword) &&
                    source.IndexOf(keyword, StringComparison.OrdinalIgnoreCase) >= 0)
                {
                    return true;
                }
            }

            return false;
        }

        sealed class MutexHandle : IDisposable
        {
            private Mutex mutex;

            public MutexHandle(Mutex mutex)
            {
                this.mutex = mutex;
            }

            public void Dispose()
            {
                if (mutex == null)
                {
                    return;
                }

                try
                {
                    mutex.ReleaseMutex();
                }
                catch (ApplicationException)
                {
                }
            }
        }
    }
}

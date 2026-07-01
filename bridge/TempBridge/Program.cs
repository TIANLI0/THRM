using System;
using System.Diagnostics;
using System.IO;
using System.IO.Pipes;
using System.Linq;
using System.Management;
using System.Runtime.InteropServices;
using System.ServiceProcess;
using System.Threading;
using Newtonsoft.Json;
using LibreHardwareMonitor.Hardware;
using LibreHardwareMonitor.PawnIo;

namespace THRM.TempBridge
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
        private const string PipeName = "THRM_TempBridge";
        private const string MutexName = @"Global\THRM_TempBridge_Singleton";
        private const int MaxInitRetries = 3;
        private const int InitRetryDelayMs = 2000;
        private const int ConsecutiveFailuresBeforeReinit = 5;
        private const int MaxReasonableTemperature = 150;
        private const int MemoryTrimIntervalSeconds = 60;
        private static Computer computer;
        private static bool running = true;
        private static readonly object lockObject = new object();
        private static Mutex singleInstanceMutex;
        private static int consecutiveFailures = 0;
        private static string lastHardwareMonitorError = string.Empty;
        private static DateTime lastMemoryTrimUtc = DateTime.MinValue;
        private static readonly IntPtr TrimWorkingSetSentinel = new IntPtr(-1);

        [DllImport("kernel32.dll")]
        private static extern bool SetProcessWorkingSetSize(IntPtr process, IntPtr minimumWorkingSetSize, IntPtr maximumWorkingSetSize);

        static void Main(string[] args)
        {
            AppDomain.CurrentDomain.UnhandledException += (_, e) =>
            {
                LogUnhandledException("AppDomain", e.ExceptionObject as Exception);
            };
            System.Threading.Tasks.TaskScheduler.UnobservedTaskException += (_, e) =>
            {
                LogUnhandledException("TaskScheduler", e.Exception);
                e.SetObserved();
            };

            bool diagnosticMode = ShouldRunDiagnosticMode(args);
            bool pipeMode = ShouldRunPipeMode(args);

            try
            {
                if (diagnosticMode)
                {
                    RunConsoleDiagnostics();
                    return;
                }

                // 初始化硬件监控
                if (!pipeMode)
                {
                    RunStdioMode();
                    return;
                }

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
                if (diagnosticMode)
                {
                    Console.Error.WriteLine("THRM TempBridge 启动失败");
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
                CloseComputerSafely("process-finally");
                if (singleInstanceMutex != null)
                {
                    singleInstanceMutex.Dispose();
                    singleInstanceMutex = null;
                }
            }
        }

        static void LogUnhandledException(string source, Exception ex)
        {
            if (ex == null)
            {
                return;
            }

            try
            {
                Console.Error.WriteLine("[fatal] " + source + ": " + ex);
                Console.Error.Flush();
            }
            catch
            {
            }
        }

        static void CloseComputerSafely(string reason)
        {
            var current = computer;
            computer = null;
            if (current == null)
            {
                return;
            }

            try
            {
                current.Close();
            }
            catch (Exception ex)
            {
                lastHardwareMonitorError = "LibreHardwareMonitor close failed (" + reason + "): " + ex.Message;
                try
                {
                    Console.Error.WriteLine("[cleanup] " + lastHardwareMonitorError);
                    Console.Error.Flush();
                }
                catch
                {
                }
            }
        }

        static void RunStdioMode()
        {
            var initStopwatch = Stopwatch.StartNew();
            LogInitProgress("stdio 模式启动，开始初始化硬件监控");
            InitializeHardwareMonitor();
            LogInitProgress(string.Format("硬件监控初始化完成，耗时 {0} ms", initStopwatch.ElapsedMilliseconds));

            using (var stdin = Console.OpenStandardInput())
            using (var stdout = Console.OpenStandardOutput())
            using (var reader = new StreamReader(stdin))
            using (var writer = new StreamWriter(stdout) { AutoFlush = true })
            {
                writer.WriteLine("READY:STDIO");
                ServeCommandLoop(reader, writer);
            }
        }

        /// <summary>
        /// 通过 stderr 输出初始化进度（以 "[init]" 前缀标记），主程序会将其记录到日志，
        /// 用于诊断部分设备上桥接启动缓慢或失败的问题。
        /// </summary>
        static void LogInitProgress(string message)
        {
            try
            {
                Console.Error.WriteLine("[init] " + message);
                Console.Error.Flush();
            }
            catch
            {
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
            if (ShouldRunPipeMode(args))
            {
                return false;
            }

            if (HasArg(args, "--diag") || HasArg(args, "--diagnose"))
            {
                return true;
            }

            return Environment.UserInteractive && !Console.IsOutputRedirected;
        }

        static bool ShouldRunPipeMode(string[] args)
        {
            return HasArg(args, "--pipe");
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
            Console.WriteLine("THRM TempBridge 诊断模式");
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

            if (computer == null)
            {
                Console.WriteLine("- LibreHardwareMonitor 未初始化，已尝试使用 Windows 温区兜底读取 CPU 温度");
                if (!string.IsNullOrWhiteSpace(lastHardwareMonitorError))
                {
                    Console.WriteLine("- 初始化信息: " + lastHardwareMonitorError);
                }
                return;
            }

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
            var pawnIoStopwatch = Stopwatch.StartNew();
            string pawnIoMessage = EnsurePawnIoReady();
            LogInitProgress(string.Format("PawnIO 检查完成，耗时 {0} ms{1}", pawnIoStopwatch.ElapsedMilliseconds,
                string.IsNullOrWhiteSpace(pawnIoMessage) ? string.Empty : "：" + pawnIoMessage));

            Exception lastException = null;
            for (int attempt = 1; attempt <= MaxInitRetries; attempt++)
            {
                var attemptStopwatch = Stopwatch.StartNew();
                try
                {
                    if (computer != null)
                    {
                        CloseComputerSafely("init-retry-reset");
                    }

                    LogInitProgress(string.Format("LibreHardwareMonitor 初始化尝试 {0}/{1}", attempt, MaxInitRetries));

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
                        lastHardwareMonitorError = string.Empty;
                        LogInitProgress(string.Format("发现有效温度传感器，初始化成功（第 {0} 次尝试，耗时 {1} ms）",
                            attempt, attemptStopwatch.ElapsedMilliseconds));
                        TrimWorkingSetIfIdle(true);
                        return;
                    }

                    lastException = new InvalidOperationException("LibreHardwareMonitor 未发现有效温度传感器");
                    LogInitProgress(string.Format("第 {0} 次尝试未发现有效温度传感器（耗时 {1} ms）",
                        attempt, attemptStopwatch.ElapsedMilliseconds));

                    // No sensors found - PawnIO may not be fully ready
                    if (attempt < MaxInitRetries)
                    {
                        CloseComputerSafely("init-no-sensors");
                        Thread.Sleep(InitRetryDelayMs);
                    }
                }
                catch (Exception ex)
                {
                    lastException = ex;
                    LogInitProgress(string.Format("第 {0} 次初始化尝试异常（耗时 {1} ms）: {2}",
                        attempt, attemptStopwatch.ElapsedMilliseconds, ex.Message));
                    if (attempt < MaxInitRetries)
                    {
                        CloseComputerSafely("init-exception");
                        Thread.Sleep(InitRetryDelayMs);
                    }
                }
            }

            // If we get here, all retries exhausted but computer may still be open
            // (just without working sensors). Keep it - it might recover on next update.
            lastHardwareMonitorError = BuildHardwareMonitorError(lastException, pawnIoMessage);
            LogInitProgress("硬件监控初始化未完全成功，将使用兜底温度读取：" + lastHardwareMonitorError);
            TrimWorkingSetIfIdle(true);
        }

        static string BuildHardwareMonitorError(Exception exception, string pawnIoMessage)
        {
            var parts = new System.Collections.Generic.List<string>();
            if (exception != null && !string.IsNullOrWhiteSpace(exception.Message))
            {
                parts.Add(exception.Message);
            }
            if (!string.IsNullOrWhiteSpace(pawnIoMessage))
            {
                parts.Add(pawnIoMessage);
            }
            if (parts.Count == 0)
            {
                parts.Add("LibreHardwareMonitor 暂未返回有效温度传感器");
            }
            return string.Join("；", parts.ToArray());
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

        static string EnsurePawnIoReady()
        {
            if (!PawnIo.IsInstalled)
            {
                return "PawnIO 驱动未安装，LibreHardwareMonitor 的部分 CPU 传感器可能不可用；已继续使用 Windows 温区兜底";
            }

            // Check PawnIO driver service is running; start it only when stopped.
            // Do not stop/restart it here because other hardware tools may share the same driver.
            try
            {
                using (var sc = new ServiceController("PawnIO"))
                {
                    if (sc.Status != ServiceControllerStatus.Running)
                    {
                        sc.Start();
                        sc.WaitForStatus(ServiceControllerStatus.Running, TimeSpan.FromSeconds(3));
                    }
                }
            }
            catch (InvalidOperationException)
            {
                // Service not found - PawnIO may use a different service name, continue
            }
            catch (System.ServiceProcess.TimeoutException)
            {
                return "PawnIO 服务启动超时，可能正被系统或其它硬件监控工具占用；已继续使用兼容温度读取";
            }
            catch (Exception ex)
            {
                return "PawnIO 服务检查失败: " + ex.Message;
            }

            return string.Empty;
        }

        static void ReinitializeHardwareMonitor()
        {
            lock (lockObject)
            {
                CloseComputerSafely("reinitialize");

                // Wait briefly after releasing handles. Avoid stopping PawnIO because other tools may share it.
                Thread.Sleep(250);

                InitializeHardwareMonitor();
            }
        }

        /// <summary>
        /// Best-effort PawnIO service check. It starts the service only when it is stopped.
        /// It intentionally does not stop/restart PawnIO because other hardware tools may share it.
        /// </summary>
        static string RestartPawnIoDriver()
        {
            return EnsurePawnIoReady();
        }

        static void TrimWorkingSetIfIdle(bool force = false)
        {
            DateTime now = DateTime.UtcNow;
            if (!force && (now - lastMemoryTrimUtc).TotalSeconds < MemoryTrimIntervalSeconds)
            {
                return;
            }

            lastMemoryTrimUtc = now;

            try
            {
                GC.Collect(0, GCCollectionMode.Optimized, false);
                using (var currentProcess = Process.GetCurrentProcess())
                {
                    SetProcessWorkingSetSize(currentProcess.Handle, TrimWorkingSetSentinel, TrimWorkingSetSentinel);
                }
            }
            catch
            {
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
                        using (var writer = new StreamWriter(pipeServer) { AutoFlush = true })
                        {
                            ServeCommandLoop(reader, writer);
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

        static void ServeCommandLoop(TextReader reader, TextWriter writer)
        {
            while (running)
            {
                try
                {
                    string commandJson = reader.ReadLine();
                    if (commandJson == null)
                    {
                        break;
                    }
                    if (string.IsNullOrWhiteSpace(commandJson))
                    {
                        continue;
                    }

                    var command = JsonConvert.DeserializeObject<Command>(commandJson) ?? new Command();
                    var response = ProcessCommand(command);

                    string responseJson = JsonConvert.SerializeObject(response);
                    writer.WriteLine(responseJson);
                    writer.Flush();
                    TrimWorkingSetIfIdle();

                    if (string.Equals(command.Type, "Exit", StringComparison.Ordinal))
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
                CloseComputerSafely("restart-pawnio");

                // 2. Ensure PawnIO is running if it is stopped, then wait after handle release.
                string pawnIoMessage = RestartPawnIoDriver();
                Thread.Sleep(250);

                // 3. Reinitialize hardware monitor with a fresh handle.
                try
                {
                    InitializeHardwareMonitor();
                    consecutiveFailures = 0;

                    // 4. Do a test read to confirm it works or that fallback can supply data.
                    var testData = GetTemperatureDataUnsafe(new TemperatureSelection());
                    if (!testData.Success && !string.IsNullOrWhiteSpace(pawnIoMessage))
                    {
                        testData.Error = string.IsNullOrWhiteSpace(testData.Error)
                            ? pawnIoMessage
                            : testData.Error + "；" + pawnIoMessage;
                    }
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

            string hardwareError = string.Empty;
            string cpuModel = string.Empty;
            string gpuModel = string.Empty;
            var cpuSensors = new System.Collections.Generic.List<TemperatureSensor>();
            var gpuCandidates = new System.Collections.Generic.List<GpuCandidate>();
            int gpuIndex = 0;

            try
            {
                if (computer != null)
                {
                    computer.Accept(new UpdateVisitor());

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
                }
                else
                {
                    hardwareError = lastHardwareMonitorError;
                }
            }
            catch (Exception ex)
            {
                hardwareError = ex.Message;
                lastHardwareMonitorError = ex.Message;
            }

            if (cpuSensors.Count == 0)
            {
                var fallbackCpuSensor = TryReadWindowsCpuTemperatureSensor();
                if (fallbackCpuSensor != null)
                {
                    cpuSensors.Add(fallbackCpuSensor);
                    if (string.IsNullOrWhiteSpace(cpuModel))
                    {
                        cpuModel = "Windows Thermal Zone";
                    }
                }
            }

            var selectedGpu = SelectGpuCandidate(gpuCandidates, selection.GpuDevice, selection.GpuSensor);
            var gpuSensors = selectedGpu != null ? selectedGpu.Sensors : new System.Collections.Generic.List<TemperatureSensor>();
            gpuModel = selectedGpu != null ? selectedGpu.Model : string.Empty;

            int cpuTemp = SelectTemperature(cpuSensors, selection.CpuSensor, new[] { "Average", "Package", "Tctl", "Tdie", "Core", "Windows" });
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
                result.Error = BuildTemperatureReadError(hardwareError);
            }
            else
            {
                result.Success = true;
                result.Error = string.Empty;
            }

            return result;
        }

        static string BuildTemperatureReadError(string hardwareError)
        {
            var parts = new System.Collections.Generic.List<string>();
            parts.Add("未读取到有效的 CPU/GPU 温度");

            string detail = !string.IsNullOrWhiteSpace(hardwareError) ? hardwareError : lastHardwareMonitorError;
            if (!string.IsNullOrWhiteSpace(detail))
            {
                parts.Add("硬件监控信息: " + detail);
            }

            parts.Add("已尝试 Windows 温区兜底；可重新初始化温度监控，或安装/更新 PawnIO 并关闭可能独占硬件传感器的软件");
            return string.Join("；", parts.ToArray());
        }

        static TemperatureSensor TryReadWindowsCpuTemperatureSensor()
        {
            int temp = TryReadPerformanceCounterCpuTemperature();
            if (temp > 0)
            {
                return new TemperatureSensor
                {
                    Key = "cpu/windows/thermal-zone",
                    Name = "Windows Thermal Zone",
                    Value = temp,
                };
            }

            temp = TryReadWmiCpuTemperature();
            if (temp > 0)
            {
                return new TemperatureSensor
                {
                    Key = "cpu/windows/wmi-thermal-zone",
                    Name = "Windows WMI Thermal Zone",
                    Value = temp,
                };
            }

            return null;
        }

        static int TryReadPerformanceCounterCpuTemperature()
        {
            const string categoryName = "Thermal Zone Information";
            const string counterName = "Temperature";
            string[] preferredInstances = new[] { @"\_TZ.THRM", "_TZ.THRM", "THRM" };

            foreach (string instance in preferredInstances)
            {
                int temp = TryReadTemperatureCounter(categoryName, counterName, instance);
                if (temp > 0)
                {
                    return temp;
                }
            }

            try
            {
                if (!PerformanceCounterCategory.Exists(categoryName))
                {
                    return 0;
                }

                var category = new PerformanceCounterCategory(categoryName);
                foreach (string instance in category.GetInstanceNames())
                {
                    int temp = TryReadTemperatureCounter(categoryName, counterName, instance);
                    if (temp > 0)
                    {
                        return temp;
                    }
                }
            }
            catch
            {
            }

            return 0;
        }

        static int TryReadTemperatureCounter(string categoryName, string counterName, string instanceName)
        {
            try
            {
                using (var counter = new PerformanceCounter(categoryName, counterName, instanceName, true))
                {
                    return NormalizeCounterTemperature(counter.NextValue());
                }
            }
            catch
            {
                return 0;
            }
        }

        static int TryReadWmiCpuTemperature()
        {
            int best = 0;
            try
            {
                using (var searcher = new ManagementObjectSearcher(@"root\WMI", "SELECT CurrentTemperature FROM MSAcpi_ThermalZoneTemperature"))
                {
                    foreach (ManagementObject obj in searcher.Get())
                    {
                        using (obj)
                        {
                            object value = obj["CurrentTemperature"];
                            if (value == null)
                            {
                                continue;
                            }

                            int temp = NormalizeWmiTemperature(Convert.ToDouble(value));
                            if (temp > best)
                            {
                                best = temp;
                            }
                        }
                    }
                }
            }
            catch
            {
            }

            return best;
        }

        static int NormalizeCounterTemperature(double raw)
        {
            if (raw <= 0)
            {
                return 0;
            }

            double celsius = raw;
            if (raw > 1000)
            {
                celsius = (raw / 10.0) - 273.15;
            }
            else if (raw > 200)
            {
                celsius = raw - 273.15;
            }

            return NormalizeCelsius(celsius);
        }

        static int NormalizeWmiTemperature(double raw)
        {
            if (raw <= 0)
            {
                return 0;
            }

            return NormalizeCelsius((raw / 10.0) - 273.15);
        }

        static int NormalizeCelsius(double celsius)
        {
            int rounded = (int)Math.Round(celsius);
            return rounded > 0 && rounded < MaxReasonableTemperature ? rounded : 0;
        }

        static TemperatureData GetTemperatureData(TemperatureSelection selection)
        {
            lock (lockObject)
            {
	                var result = GetTemperatureDataUnsafe(selection);

                if (!result.Success || (result.CpuTemp == 0 && result.GpuTemp == 0))
                {
                    consecutiveFailures++;

                    // Auto-reinitialize after consecutive failures. This refreshes LHM/PawnIO handles
                    // without stopping the shared PawnIO driver service.
                    if (consecutiveFailures >= ConsecutiveFailuresBeforeReinit)
                    {
                        consecutiveFailures = 0;
                        result.Error = "连续读取失败，正在重新初始化温度监控并重新获取硬件句柄...";

                        ThreadPool.QueueUserWorkItem(_ =>
                        {
                            try { ReinitializeHardwareMonitor(); }
                            catch { }
                        });
                    }
                    else if (string.IsNullOrEmpty(result.Error))
                    {
                        result.Error = string.Format(
                            "未读取到有效的 CPU/GPU 温度（连续失败 {0}/{1}，达到阈值后将自动重新初始化温度监控）",
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

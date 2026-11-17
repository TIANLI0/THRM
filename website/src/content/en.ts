import type { Dict } from "./types";

export const en: Dict = {
  locale: "en",
  htmlLang: "en",
  meta: {
    title: "THRM · Control center for Flydigi BS-series coolers",
    description:
      "THRM is an open-source, third-party controller for Flydigi BS-series laptop coolers: editable fan curves, SmartControl, noise calibration, RGB lighting and hardware monitoring on Windows and Linux.",
  },
  nav: {
    features: "Features",
    devices: "Devices",
    download: "Download",
    faq: "FAQ",
    changelog: "Changelog",
    github: "GitHub",
    downloadCta: "Download",
    openMenu: "Open menu",
    closeMenu: "Close menu",
    switchLanguage: "切换到简体中文",
    languageLabel: "中文",
    theme: { system: "System", light: "Light", dark: "Dark", label: "Switch theme, current: " },
  },
  hero: {
    release: "v{version} is out",
    title: ["Light.", "Simple.", "Yours."],
    lead: "A third-party cooling driver built for gaming laptops. THRM drives your Flydigi BS-series cooler from live CPU and GPU temperatures, and keeps working from the tray after you close the window.",
    downloadWindows: "Download for Windows",
    downloadLinux: "Download for Linux",
    source: "View source",
    platforms: ["Windows 10 / 11", "Linux x86_64", "Free and MIT licensed"],
    demoHint: "Drag the red line on the curve to see the fan respond",
  },
  window: {
    sidebar: { status: "Status", curve: "Curve", control: "Settings", about: "About", expand: "Expand sidebar" },
    connected: "Connected",
    smartControl: "Smart control",
    modeLine: "Smart control · Adjusts fan speed automatically based on real-time temperature",
    disconnect: "Disconnect",
    cpuTemperature: "CPU temperature",
    gpuTemperature: "GPU temperature",
    fanRpm: "Fan RPM",
    tempStatus: { overheat: "Overheating", high: "High", normal: "Normal", good: "Good" },
    targetSummary: "Target {target} · {gear}",
    gears: ["Quiet", "Standard", "Strong", "Overclock"],
    controlProtection: "Control and protection",
    stats: {
      controlMode: "Control mode",
      controlModeValue: "Smart control (Default)",
      maxRpm: "Maximum RPM",
      workMode: "Work mode",
      workModeValue: "Auto mode (live RPM)",
      cpuPower: "CPU power",
      gpuPower: "GPU power",
    },
    curveTitle: "Fan speed curve",
    curveSlider: "Simulated temperature",
    historyTitle: "Temperature and fan history",
    historyBadge: "Background",
  },
  pipeline: {
    title: "From one temperature reading to one change in fan speed",
    lead: "THRM Core runs as its own background service and takes every sample through these four steps, whether the main window is open or not.",
    steps: [
      {
        title: "Read temperatures",
        body: "Control from CPU, GPU or whichever is hotter. Pick specific sensors, or average several CPU sensors.",
      },
      {
        title: "Look up the curve",
        body: "Interpolate on the temperature-to-RPM curve you drew, switching profiles by weekday and time.",
      },
      {
        title: "Smooth it out",
        body: "SmartControl filters isolated spikes, applies hysteresis and ramp limits, and boosts early when power draw climbs.",
      },
      {
        title: "Send to the cooler",
        body: "Speed goes out over HID or BLE. When the HID interface comes back, THRM reconnects on its own.",
      },
    ],
  },
  features: {
    title: "Everything past the fixed fan steps",
    lead: "Basic control gives you a few presets. THRM opens up curves, learning, noise and lighting, and does all of it on your machine.",
    items: {
      curve: {
        title: "Fan curves and profiles",
        body: "Edit the temperature-to-RPM curve visually, keep several profiles, share them as profile codes, or switch them on a weekly schedule.",
      },
      smart: {
        title: "SmartControl",
        body: "Refines your curve from long-term results. Pick a learning bias, and clear what it learned with one click whenever you like.",
      },
      noise: {
        title: "Noise calibration",
        body: "Measures relative noise at each RPM with your microphone, finds noise knees and likely resonance bands, and quiet learning steers around them.",
      },
      monitor: {
        title: "Hardware monitoring",
        body: "History for CPU and GPU temperature and power, plus current and target fan speed. On Windows, fan RPM can show up in the RTSS in-game overlay.",
      },
      lighting: {
        title: "RGB lighting",
        body: "Six effects with adjustable brightness and speed. Fans stop and lights go off when the system sleeps or hibernates.",
      },
      desktop: {
        title: "On the desktop",
        body: "Stays quietly in the background, like a tool that shipped with your system.",
      },
    },
    schedule: {
      days: ["M", "T", "W", "T", "F", "S", "S"],
      profiles: ["Quiet", "Balanced", "Gaming"],
    },
    smart: {
      tendencies: ["Balanced", "Cooling first", "Quiet first"],
      base: "Base curve",
      learned: "Learned",
    },
    noise: { resonance: "Likely resonance", axis: "Fan RPM" },
    monitor: { cpu: "CPU temp", gpu: "GPU temp", fan: "Fan RPM", overlay: "In-game overlay" },
    lighting: { modes: ["Temperature", "Solid", "Multicolor", "Rotate", "Flow", "Breathe"] },
    desktop: [
      "Lives in the tray and starts with your system",
      "Global hotkeys for gears, smart control and profiles",
      "Light, dark, follow-system and custom themes",
      "Windows Acrylic, Mica and Mica Alt materials",
      "简体中文, English and 日本語",
      "In-app updates and one-click diagnostics export",
    ],
  },
  devices: {
    title: "Supported devices",
    lead: "Maximum speed and lighting vary by firmware and hardware revision, so THRM works from what each device reports.",
    pairFirst: "Pair it in system Bluetooth settings first",
    autoScan: "THRM scans and connects once it's powered on",
  },
  quickstart: {
    title: "Start cooling in four steps",
    steps: [
      { title: "Power the cooler", body: "Plug it in and make sure it's running." },
      {
        title: "Connect it",
        body: "BS1 just needs to be advertising over Bluetooth. Pair BS2, BS2 Pro, BS3 and BS3 Pro in system Bluetooth settings first.",
      },
      { title: "Launch THRM", body: "Check that the model, temperatures and fan speed all show up on the status page." },
      {
        title: "Choose how it runs",
        body: "Turn on smart control, or pick a profile on the curve page. Close the window and THRM keeps cooling from the tray.",
      },
    ],
  },
  download: {
    title: "Download THRM",
    tablist: "Choose your system",
    windowsReq: "Windows 10 / 11 x64",
    linuxReq: "x86_64; Debian, Ubuntu and derivatives recommended",
    recommended: "Recommended",
    installer: {
      name: "Installer",
      body: "Right for most people. Installs and starts the core service and temperature bridge for you.",
    },
    portable: {
      name: "Portable",
      body: "Extract everything and run THRM.exe. Keep THRM Core.exe and the bridge folder next to it.",
    },
    linux: [
      {
        key: "deb",
        label: "Debian / Ubuntu",
        body: "Installs the app, core service, desktop entry and udev rules for Flydigi HID devices.",
        install: "sudo apt install ./{file}",
      },
      {
        key: "arch",
        label: "Arch / CachyOS / Manjaro",
        body: "Installs the binaries CI already built; no local Go or Bun needed.",
        install: "sudo pacman -U ./{file}",
      },
      {
        key: "linuxPortable",
        label: "Portable archive",
        body: "Installs to ~/.local/bin by default.",
        install: "tar -xzf {file}\ncd THRM-linux-amd64\nchmod +x install.sh && ./install.sh",
      },
    ],
    linuxDeps: "Requires GTK 3, WebKit2GTK 4.1, hidapi and udev. BS1 also needs a running BlueZ service.",
    signing:
      "Windows releases are code-signed with a Certum certificate by GitHub Actions, right after they are built from the public source. Only download from GitHub Releases.",
    releases: "All releases",
    changelog: "Changelog",
    copy: "Copy command",
    copied: "Copied",
    get: "Download",
  },
  changelog: {
    title: "Changelog",
    lead: "What changed in each version, with its packages. Files are served through this site, so you don't need GitHub access.",
    latest: "Latest",
    prerelease: "Pre-release",
    publishedOn: "Published {date}",
    downloads: "Packages",
    noNotes: "No release notes for this version.",
    unavailable: "The release list isn't available right now. Refresh in a moment.",
    viewOnGithub: "View on GitHub",
  },
  faq: {
    title: "FAQ",
    items: [
      {
        q: "The cooler won't connect",
        a: [
          "Make sure it's powered and that the model matches the connection type. BS2, BS2 Pro, BS3 and BS3 Pro must be paired in system Bluetooth settings first.",
          "Close the official Flydigi app or any other tool that may hold the device. If it still fails, remove the Bluetooth pairing and pair again, or unplug the cooler for a few seconds. If BS1 doesn't show up, long-press its button to start advertising again.",
        ],
      },
      {
        q: "CPU or GPU temperature shows 0",
        a: [
          "On Windows, try in order: reinitialize temperature monitoring in Settings, close other monitoring or fan tools, reinstall or update PawnIO, restart, then run THRM as administrator.",
          "On hybrid-graphics laptops that only need CPU temperature, turn off GPU monitoring so background polling doesn't wake the discrete GPU.",
        ],
      },
      {
        q: "THRM keeps running after I close the window",
        a: ["That's so it can keep controlling the fan. THRM minimizes to the tray by default; choose Quit from the tray icon menu to exit completely."],
      },
      {
        q: "Can I see the cooler's fan speed in games?",
        a: [
          "Yes, on Windows, with RivaTuner Statistics Server (RTSS) installed. MSI Afterburner isn't needed.",
          "Turn on RTSS OSD in THRM's settings and the overlay shows Cooler Fan: <rpm> RPM. Refresh can be 250 ms to 2 s and doesn't affect how fast control responds.",
        ],
      },
      {
        q: "Is THRM made by Flydigi?",
        a: [
          "No. THRM is a community-built, third-party open-source project with no affiliation to Flydigi.",
          "Custom speeds, unofficial minimum RPM and experimental features may carry extra risk. Use them with your device's limits in mind.",
        ],
      },
      {
        q: "How do I report a problem?",
        a: [
          "Open a GitHub issue with your THRM version, OS version, cooler model, steps to reproduce and the diagnostics package exported from Settings.",
          "Diagnostics may include device and hardware details, so review them before uploading.",
        ],
      },
    ],
  },
  footer: {
    disclaimer: "THRM is a community-built, third-party open-source project, not affiliated with Flydigi.",
    links: {
      github: "Source code",
      releases: "Releases",
      issues: "Issues",
      privacy: "Privacy",
      signing: "Code signing policy",
      license: "MIT License",
    },
    copyright: "© 2026 TIANLI0",
  },
};

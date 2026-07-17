import type { Metadata } from "next";
// 本地字体：仅打包体积很小的拉丁字体，避免构建时联网请求 Google 字体失败。
// 中文（CJK）字体不打包——体积高达数十 MB，改用系统自带中文字体（见 globals.css 的 --font-ui-cjk）。
import "@fontsource-variable/manrope/index.css";
import "@fontsource-variable/geist-mono/index.css";
import "./globals.css";
import SystemThemeSync from "./components/SystemThemeSync";
import { Toaster } from "@/components/ui/sonner";
import { TooltipProvider } from "@/components/ui/tooltip";
import { BRAND } from "./lib/brand";
import { AppI18nProvider } from "./lib/i18n";
import { getThemeBootstrapScript } from "./lib/theme-bootstrap";

export const metadata: Metadata = {
  title: BRAND.name,
  description: BRAND.description,
};

export default function RootLayout({
  children,
}: Readonly<{
  children: React.ReactNode;
}>) {
  const themeBootstrapScript = getThemeBootstrapScript();

  return (
    <html lang="zh-CN" suppressHydrationWarning>
      <head>
        <script
          id="thrm-theme-bootstrap"
          dangerouslySetInnerHTML={{ __html: themeBootstrapScript }}
        />
      </head>
      <body>
        <AppI18nProvider>
          <SystemThemeSync />
          <TooltipProvider delayDuration={180}>
            {children}
            <Toaster richColors closeButton position="top-right" />
          </TooltipProvider>
        </AppI18nProvider>
      </body>
    </html>
  );
}

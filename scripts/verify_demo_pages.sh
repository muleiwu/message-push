#!/bin/sh
set -eu

session="message-push-demo-verify"
root="$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)"

cleanup() {
  playwright-cli -s="$session" close >/dev/null 2>&1 || true
}
trap cleanup EXIT INT TERM

playwright-cli -s="$session" open about:blank
verification_output="$(playwright-cli -s="$session" run-code 'async (page) => {
  const base = "http://localhost:8080/admin/";
  const shot = "/Users/hepeichun/Code/cnb.cool/mliev/push/message-push/docs/manual/zh-CN/assets/screenshots";
  const apiErrors = [];
  const consoleErrors = [];
  page.on("response", (response) => {
    if (response.url().includes("/api/") && response.status() >= 400) {
      apiErrors.push(`${response.status()} ${response.request().method()} ${response.url()}`);
    }
  });
  page.on("console", (message) => {
    if (message.type() === "error") consoleErrors.push(message.text());
  });
  page.on("pageerror", (error) => consoleErrors.push(error.message));
  await page.route("https://api.unisvg.com/**", async (route) => {
    const rawURL = route.request().url();
    const iconsMatch = rawURL.match(/[?&]icons=([^&]+)/);
    const names = (iconsMatch ? iconsMatch[1].replaceAll("%2C", ",") : "circle").split(",");
    const icons = {};
    for (const name of names) {
      icons[name] = { body: `<circle cx="12" cy="12" r="8" fill="currentColor"/>` };
    }
    const prefix = rawURL.split("?")[0].split("/").pop().replace(".json", "") || "icon";
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({ prefix, icons, width: 24, height: 24 }),
    });
  });
  await page.setViewportSize({ width: 1440, height: 900 });
  await page.goto(base + "#/auth/login");
  await page.locator("input").first().fill("demo-admin");
  await page.locator("input[type=password]").fill("demo-pass-2026");
  await page.locator("input[type=password]").press("Enter");
  await page.waitForURL(/#\/dashboard/);
  await page.waitForTimeout(3500);

  const routes = [
    ["/dashboard", "发送准备台"],
    ["/applications", "应用管理"],
    ["/templates/message-templates", "系统模板管理"],
    ["/providers-manage/accounts", "服务商账号配置"],
    ["/providers-manage/list", "服务商列表"],
    ["/providers-manage/channel-types", "渠道类型"],
    ["/templates/provider-templates", "供应商模板管理"],
    ["/provider-signatures", "供应商签名"],
    ["/channels", "发送通道"],
    ["/channels/new", "新建发送通道"],
    ["/channels/1", "登录验证码短信（主备就绪）"],
    ["/failure-rules", "失败规则管理"],
    ["/monitoring/push-tasks", "推送任务"],
    ["/monitoring/batch-tasks", "批量任务"],
    ["/monitoring/sms-replies", "上行短信"],
    ["/analytics", "统计分析"],
    ["/users", "管理员账号"],
  ];
  const checked = [];
  for (const [route, readyText] of routes) {
    await page.goto(base + "#" + route);
    await page.waitForLoadState("networkidle");
    await page.getByText(readyText, { exact: false }).first().waitFor({
      state: "visible",
      timeout: 15000,
    });
    // The route spinner has a 500 ms minimum display time and a 500 ms fade-out.
    // Network idleness alone can therefore still capture the previous page beneath it.
    await page.waitForTimeout(75);
    await page.locator(".loader").waitFor({ state: "detached", timeout: 5000 });
    await page.waitForTimeout(150);
    const body = await page.locator("body").innerText();
    if (/404|页面不存在|Internal Server Error/i.test(body)) {
      throw new Error(`页面 ${route} 呈现错误状态`);
    }
    checked.push(route);
    if (route === "/dashboard") {
      await page.screenshot({ path: `${shot}/dashboard/01-onboarding.png` });
    } else if (route === "/channels") {
      await page.screenshot({ path: `${shot}/channels/01-list.png` });
    } else if (route === "/failure-rules") {
      await page.screenshot({ path: `${shot}/failure-rules/01-list.png` });
      await page.getByText("编辑", { exact: true }).first().click();
      await page.getByText("编辑规则", { exact: true }).waitFor();
      await page.waitForTimeout(500);
      await page.screenshot({ path: `${shot}/failure-rules/02-edit.png` });
      await page.keyboard.press("Escape");
    } else if (route === "/monitoring/batch-tasks") {
      await page.screenshot({ path: `${shot}/batch-tasks/01-list.png` });
      await page.getByText("查看详情", { exact: true }).first().click();
      await page.getByText("批次详情", { exact: true }).waitFor();
      await page.waitForTimeout(500);
      await page.screenshot({ path: `${shot}/batch-tasks/02-detail.png` });
      await page.keyboard.press("Escape");
    } else if (route === "/users") {
      await page.screenshot({ path: `${shot}/admin-users/01-list.png` });
      await page.getByRole("button", { name: /编辑管理员/ }).first().click();
      await page.getByText("编辑管理员", { exact: true }).waitFor();
      await page.waitForTimeout(500);
      await page.screenshot({ path: `${shot}/admin-users/02-edit.png` });
      await page.keyboard.press("Escape");
    }
  }
  await page.waitForTimeout(500);
  if (apiErrors.length || consoleErrors.length) {
    throw new Error(JSON.stringify({ apiErrors, consoleErrors }));
  }
  return JSON.stringify({ checkedPages: checked.length + 1, adminRoutes: checked });
}')"
printf '%s\n' "$verification_output"
case "$verification_output" in
  *"### Error"*) exit 1 ;;
esac

printf 'Playwright demo verification completed from %s\n' "$root"

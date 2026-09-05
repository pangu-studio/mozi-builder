import { test, expect } from "@playwright/test";
test("existing designer mounts; dialogs survive unmount/remount and theme changes", async ({
  page,
}) => {
  const errors: string[] = [];
  page.on("pageerror", (error) => errors.push(error.message));
  await page.goto("/#designer");
  await expect(page.getByText("设计器已挂载", { exact: true })).toBeVisible();
  await expect(
    page.getByRole("heading", { name: "Mozi v2 设计器", exact: true }),
  ).toBeVisible();
  await page.getByRole("link", { name: "交互检查" }).click();
  await page.getByRole("button", { name: "打开弹窗" }).click();
  await expect(page.getByText("弹窗保留在设计器容器内。")).toBeVisible();
  await page.getByRole("button", { name: "确 定" }).click();
  await page.getByRole("button", { name: "显示通知" }).click();
  await expect(page.getByText("设计器通知正常")).toBeVisible();
  await page.getByRole("button", { name: "卸载设计器" }).click();
  await expect(page.getByTestId("designer-app")).toHaveCount(0);
  await page.getByRole("button", { name: "挂载设计器" }).click();
  await expect(page.getByTestId("designer-app")).toBeVisible();
  await page.getByRole("button", { name: "切换主题" }).click();
  await expect(page.getByTestId("designer-app")).toHaveClass(/dark/);
  await page.getByRole("link", { name: "交互检查" }).click();
  await page.getByRole("button", { name: "显示通知" }).click();
  await expect(page.getByText("设计器通知正常")).toBeVisible();
  await page.getByRole("link", { name: "运行管理", exact: true }).click();
  await expect(page.getByTestId("designer-app")).toBeHidden();
  await page.goBack();
  await expect(page.getByTestId("designer-app")).toBeVisible();
  await page.reload();
  await expect(page.getByTestId("designer-app")).toBeVisible();
  expect(errors).toEqual([]);
});

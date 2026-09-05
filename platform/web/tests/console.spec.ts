import { test, expect, type Page } from "@playwright/test";
const token = "test-session-token";
async function mock(
  page: Page,
  options: {
    empty?: boolean;
    expired?: boolean;
    viewer?: boolean;
    conflict?: boolean;
    projectFailure?: boolean;
  } = {},
) {
  const projects = options.empty
    ? []
    : [
        {
          id: "alpha",
          name: "Alpha 项目",
          slug: "alpha",
          role: options.viewer ? "viewer" : "owner",
        },
        { id: "beta", name: "Beta 项目", slug: "beta", role: "viewer" },
      ];
  const environments: Record<string, unknown[]> = {
    alpha: [
      {
        id: "dev",
        name: "Alpha 开发",
        slug: "development",
        kind: "development",
      },
    ],
    beta: [
      { id: "prod", name: "Beta 生产", slug: "production", kind: "production" },
    ],
  };
  await page.route("**/api/v2/**", async (route) => {
    const req = route.request(),
      path = new URL(req.url()).pathname.replace("/api/v2", "");
    const send = (data: unknown, status = 200) =>
      route.fulfill({
        status,
        contentType: "application/json",
        body: JSON.stringify(data),
      });
    if (path === "/login") {
      expect(req.postDataJSON()).toEqual({
        email: "owner@example.com",
        password: "password-1234",
      });
      return send({ access_token: token });
    }
    expect(req.headers().authorization).toBe(`Bearer ${token}`);
    if (path === "/logout") return route.fulfill({ status: 204 });
    if (options.expired) return send({ error: "unauthorized" }, 401);
    if (path === "/me")
      return send({
        id: "user",
        email: "owner@example.com",
        display_name: "测试用户",
      });
    if (path === "/projects") {
      if (options.projectFailure) return send({ error: "failed" }, 500);
      if (req.method() === "POST") {
        const body = req.postDataJSON();
        const p = { id: "created", ...body, role: "owner" };
        projects.push(p);
        environments.created = [];
        return send({ id: p.id }, 201);
      }
      return send(projects);
    }
    const id = path.split("/")[2];
    if (path.endsWith("/environments")) {
      if (req.method() === "POST") {
        if (options.conflict) return send({ error: "conflict" }, 409);
        const body = req.postDataJSON();
        environments[id].push({ id: "created-env", ...body });
        return send({ id: "created-env" }, 201);
      }
      return send(environments[id] || []);
    }
    return send({}, 404);
  });
}
async function login(page: Page) {
  await page.goto("/");
  await page.getByLabel("邮箱", { exact: true }).fill("owner@example.com");
  await page.getByLabel("密码", { exact: true }).fill("password-1234");
  await page.getByRole("button", { name: "登录平台" }).click();
}
async function switchProject(page: Page) {
  await page.getByRole("combobox", { name: "当前项目" }).click();
  await page.getByTitle("Beta 项目", { exact: true }).click();
}

test("login, create environment, switch project, reload and logout", async ({
  page,
}) => {
  await mock(page);
  await page.goto("/");
  await page.screenshot({
    path: test.info().outputPath("login.png"),
    fullPage: true,
  });
  await login(page);
  await expect(page.getByText("Alpha 开发", { exact: true })).toBeVisible();
  await page.getByRole("button", { name: "新建环境", exact: true }).click();
  await page.getByLabel("名称", { exact: true }).fill("集成验证");
  await page.getByLabel("标识", { exact: true }).fill("integration");
  await page.getByRole("button", { name: /^创\s*建$/ }).click();
  await expect(page.getByText("集成验证", { exact: true })).toBeVisible();
  await page.screenshot({
    path: test.info().outputPath("environments.png"),
    fullPage: true,
  });
  await switchProject(page);
  await expect(page.getByText("Beta 生产", { exact: true })).toBeVisible();
  await expect(page.getByText("Alpha 开发", { exact: true })).toHaveCount(0);
  await expect(
    page.getByRole("button", { name: "新建环境", exact: true }),
  ).toHaveCount(0);
  await expect(page.getByText("受保护", { exact: true })).toBeVisible();
  await page.reload();
  await expect(
    page.getByRole("heading", { name: "环境管理", exact: true }),
  ).toBeVisible();
  await page.getByRole("button", { name: "退出登录" }).click();
  await expect(
    page.getByRole("heading", { name: "登录开发平台" }),
  ).toBeVisible();
  expect(
    await page.evaluate(() => sessionStorage.getItem("mozi_v2_session")),
  ).toBeNull();
});
test("empty account creates its first project", async ({ page }) => {
  await mock(page, { empty: true });
  await login(page);
  await page.getByRole("button", { name: "创建第一个项目" }).click();
  await page.getByLabel("名称", { exact: true }).fill("新项目");
  await page.getByLabel("标识", { exact: true }).fill("my-project");
  await page.getByRole("button", { name: /^创\s*建$/ }).click();
  await expect(page.getByText("暂无环境", { exact: true })).toBeVisible();
  await expect(
    page.getByRole("button", { name: "新建环境", exact: true }),
  ).toBeVisible();
});
test("expired stored session returns to login", async ({ page }) => {
  await mock(page, { expired: true });
  await page.addInitScript(() =>
    sessionStorage.setItem("mozi_v2_session", "test-session-token"),
  );
  await page.goto("/");
  await expect(
    page.getByText("登录已失效，请重新登录。", { exact: true }),
  ).toBeVisible();
  await expect(
    page.getByRole("heading", { name: "登录开发平台" }),
  ).toBeVisible();
});
test("conflict keeps form values and shows actionable error", async ({
  page,
}) => {
  await mock(page, { conflict: true });
  await login(page);
  await page.getByRole("button", { name: "新建环境", exact: true }).click();
  await page.getByLabel("名称", { exact: true }).fill("开发");
  await page.getByLabel("标识", { exact: true }).fill("development");
  await page.getByRole("button", { name: /^创\s*建$/ }).click();
  await expect(page.getByText("标识已存在，请使用其他标识。")).toBeVisible();
  await expect(page.getByLabel("标识", { exact: true })).toHaveValue(
    "development",
  );
});
test("late response cannot leak prior project environments", async ({
  page,
}) => {
  await mock(page);
  let release!: () => void;
  const gate = new Promise<void>((resolve) => (release = resolve));
  await page.route("**/api/v2/projects/alpha/environments", async (route) => {
    await gate;
    await route.fulfill({
      json: [
        {
          id: "old",
          name: "过时的 Alpha 环境",
          slug: "old",
          kind: "development",
        },
      ],
    });
  });
  await login(page);
  await expect(
    page.getByRole("heading", { name: "环境管理", exact: true }),
  ).toBeVisible();
  await switchProject(page);
  await expect(page.getByText("Beta 生产", { exact: true })).toBeVisible();
  release();
  await expect(
    page.getByText("过时的 Alpha 环境", { exact: true }),
  ).toHaveCount(0);
});
test("project load failure offers retry and no stale content", async ({
  page,
}) => {
  const options = { projectFailure: true };
  await mock(page, options);
  await login(page);
  await expect(page.getByText("服务暂时不可用，请稍后重试。")).toBeVisible();
  options.projectFailure = false;
  await page.getByRole("button", { name: /^重\s*试$/ }).click();
  await expect(page.getByText("Alpha 开发", { exact: true })).toBeVisible();
});
test("designer mounts after login and logout destroys the child", async ({
  page,
}) => {
  const errors: string[] = [];
  page.on("pageerror", (e) => errors.push(e.message));
  await mock(page);
  await login(page);
  await page.getByRole("link", { name: "模型设计", exact: true }).click();
  await expect(page.getByText("设计器已挂载", { exact: true })).toBeVisible();
  await expect(page.getByTestId("designer-app")).toBeVisible();
  await page.getByRole("button", { name: "退出登录" }).click();
  await expect(page.getByTestId("designer-app")).toHaveCount(0);
  expect(errors).toEqual([]);
});

test("mobile layout remains usable", async ({ page }) => {
  await page.setViewportSize({ width: 390, height: 844 });
  await mock(page);
  await login(page);
  await expect(page.getByText("Alpha 开发", { exact: true })).toBeVisible();
  expect(
    await page.evaluate(
      () => document.documentElement.scrollWidth <= window.innerWidth,
    ),
  ).toBe(true);
  await page.screenshot({
    path: test.info().outputPath("mobile.png"),
    fullPage: true,
  });
});

test("project model create, conflict preservation, history and project switch", async ({
  page,
}) => {
  const modelErrors: string[] = [];
  page.on("pageerror", (e) => modelErrors.push(e.message));
  await mock(page);
  type Entry = {
    module: string;
    name: string;
    version: string;
    document: Record<string, unknown>;
  };
  const records: Record<string, Entry[]> = {
    alpha: [],
    beta: [
      {
        module: "content",
        name: "Card",
        version: "b1",
        document: {
          module: "content",
          model: "Card",
          label: "Beta 模型",
          table: "cards",
          fields: [],
          semantics: { purpose: "Beta" },
        },
      },
    ],
  };
  let conflict = false;
  await page.route("**/api/v2/projects/*/design/models**", async (route) => {
    const req = route.request();
    const parts = new URL(req.url()).pathname.split("/");
    const project = parts[4];
    const name = parts[8];
    if (req.method() === "POST") {
      const input = req.postDataJSON();
      const m = {
        module: input.document.module,
        name: input.document.model,
        version: "a1",
        document: input.document,
      };
      records[project].push(m);
      return route.fulfill({ status: 201, json: m });
    }
    const model = records[project].find((m) => m.name === name);
    if (req.method() === "PUT") {
      if (conflict)
        return route.fulfill({
          status: 409,
          json: { error: "model_conflict" },
        });
      return route.fulfill({ json: model });
    }
    if (parts.at(-1) === "history")
      return route.fulfill({
        json: [
          {
            version: "a1",
            document: model!.document,
            action: "created",
            created_at: "2026-09-05T12:00:00Z",
            actor_id: "user",
          },
        ],
      });
    return route.fulfill({ json: name ? model : records[project] });
  });
  await login(page);
  await page.getByRole("link", { name: "模型设计", exact: true }).click();
  await expect(page.getByTestId("real-designer")).toBeVisible();
  await page.getByRole("button", { name: "新建模型", exact: true }).click();
  await page
    .getByRole("textbox", { name: "模型标识", exact: true })
    .fill("Card");
  await page
    .getByRole("textbox", { name: "模型名称", exact: true })
    .fill("Alpha 模型");
  await page
    .getByRole("textbox", { name: "数据表名", exact: true })
    .fill("cards");
  await page.getByRole("button", { name: "保存模型", exact: true }).click();
  await expect(
    page.getByText("模型已保存，历史版本已记录。", { exact: true }),
  ).toBeVisible();
  conflict = true;
  await page
    .getByRole("textbox", { name: "模型名称", exact: true })
    .fill("本地尚未保存");
  await page.getByRole("button", { name: "保存模型", exact: true }).click();
  await expect(page.getByText(/保存冲突：模型已被修改/)).toBeVisible();
  await expect(
    page.getByRole("textbox", { name: "模型名称", exact: true }),
  ).toHaveValue("本地尚未保存");
  await page.getByRole("button", { name: "版本历史", exact: true }).click();
  await expect(page.getByText(/created · a1/)).toBeVisible();
  await page.getByRole("button", { name: "Close", exact: true }).click();
 await expect(page.getByRole("dialog",{name:"模型历史"})).toBeHidden();
  await page.screenshot({
    path: test.info().outputPath("models.png"),
    fullPage: true,
  });
  page.once("dialog", (dialog) => dialog.accept());
  await switchProject(page);
  await expect(page.getByText("Beta 模型", { exact: true })).toBeVisible();
  await expect(
    page.getByRole("button", { name: "新建模型", exact: true }),
  ).toHaveCount(0);
  await expect(
    page.getByRole("textbox", { name: "模型名称", exact: true }),
  ).toHaveCount(0);
  await page.getByRole("button", { name: "Card", exact: true }).click();
  await expect(
    page.getByRole("textbox", { name: "模型名称", exact: true }),
  ).toHaveValue("Beta 模型");
  await expect(
    page.getByRole("textbox", { name: "模型名称", exact: true }),
  ).toBeDisabled();
  expect(modelErrors).toEqual([]);
});

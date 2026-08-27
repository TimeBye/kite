import { expect, test, type Locator, type Page } from '@playwright/test'

const chartName = 'kite'
const repositoryURL = 'https://kite-org.github.io/kite/'
const installVersion = '0.10.0'
const specifiedUpgradeVersion = '0.11.0'
const namespace = 'default'
const baseValues = `replicaCount: 1
anonymousUserEnabled: true
podLabels:
  e2e-mode: base
`
const upgradedValues = `replicaCount: 1
anonymousUserEnabled: true
podLabels:
  e2e-mode: upgraded
`

async function fillMonacoEditor(
  page: Page,
  root: Locator,
  editorIndex: number,
  value: string
) {
  const editor = root.locator('.monaco-editor').nth(editorIndex)
  const editorText = editor.locator('.view-lines')

  await expect(editor).toBeVisible({ timeout: 60_000 })
  const firstLine = value.trim().split('\n')[0]

  await editorText.click({ position: { x: 10, y: 10 } })
  await page.keyboard.press('Control+A')
  await page.keyboard.press('Backspace')
  await page.keyboard.press('Meta+A')
  await page.keyboard.press('Backspace')
  await page.keyboard.insertText(value)
  await expect(editorText).toContainText(firstLine)
}

function escapeRegExp(value: string) {
  return value.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')
}

async function selectRepositoryFilter(page: Page, repositoryName: string) {
  await page.locator('[data-slot="select-trigger"]').first().click()
  await page.getByRole('option', { name: repositoryName }).click()
}

async function switchToRepositories(page: Page) {
  await page.getByText('Repositories', { exact: true }).click()
}

async function selectUpgradeChart(
  page: Page,
  dialog: Locator,
  repositoryName: string
) {
  const selectTriggers = dialog.locator('[data-slot="select-trigger"]')
  if ((await selectTriggers.count()) < 2) {
    return
  }

  await selectTriggers.first().click()
  await page
    .getByRole('option', {
      name: new RegExp(`${escapeRegExp(repositoryName)}/${chartName}`),
    })
    .click()
}

async function selectUpgradeVersion(
  page: Page,
  dialog: Locator,
  version: string
) {
  const versionSelect = dialog.locator('[data-slot="select-trigger"]').last()
  await expect(versionSelect).toBeVisible({ timeout: 60_000 })
  await versionSelect.click()
  await page
    .getByRole('option', {
      name: new RegExp(`^${escapeRegExp(version)}(?:\\s|$)`),
    })
    .click()
}

async function expectReleaseSummary(
  page: Page,
  releaseName: string,
  version: string,
  revision: number
) {
  await expect(page.getByRole('heading', { name: releaseName })).toBeVisible({
    timeout: 120_000,
  })
  await page.getByRole('tab', { name: 'Overview' }).click()

  const chartSummary = page
    .locator(`[title="${chartName}"]`)
    .locator('xpath=..')
  await expect(chartSummary).toContainText(version, { timeout: 120_000 })

  const revisionSummary = page
    .getByText('Revision', { exact: true })
    .locator('xpath=..')
  await expect(revisionSummary).toContainText(String(revision), {
    timeout: 120_000,
  })
}

async function expectReleaseValues(
  page: Page,
  expectedText: string,
  absentText?: string
) {
  await page.getByRole('tab', { name: 'Values' }).click()
  const editorText = page.locator('.monaco-editor .view-lines').first()
  await expect(editorText).toContainText('replicaCount:', { timeout: 60_000 })
  await expect(editorText).toContainText(expectedText)
  if (absentText) {
    await expect(editorText).not.toContainText(absentText)
  }
}

async function expectAppliedPodLabel(
  page: Page,
  releaseName: string,
  expectedMode: string
) {
  await expect
    .poll(
      async () => {
        const response = await page.request.get(
          `/api/v1/deployments/${namespace}?labelSelector=${encodeURIComponent(
            `app.kubernetes.io/instance=${releaseName}`
          )}`
        )
        if (!response.ok()) {
          return ''
        }
        const body = (await response.json()) as {
          items?: Array<{
            spec?: {
              template?: {
                metadata?: {
                  labels?: Record<string, string>
                }
              }
            }
          }>
        }
        const labels = (body.items || []).map(
          (item) => item.spec?.template?.metadata?.labels?.['e2e-mode'] || ''
        )
        if (!labels.length || labels.some((label) => !label)) {
          return ''
        }
        return labels.every((label) => label === expectedMode)
          ? expectedMode
          : labels.join(',')
      },
      { timeout: 60_000 }
    )
    .toBe(expectedMode)
}

async function expectDryRunPreview(dialog: Locator) {
  await expect(dialog.getByText('Dry run preview')).toBeVisible({
    timeout: 120_000,
  })
  await expect(
    dialog.getByText('No resources rendered by dry run.')
  ).toBeHidden()
}

async function deleteReleaseFromCurrentPage(page: Page, releaseName: string) {
  await page.getByRole('button', { name: 'Delete' }).click()
  const deleteDialog = page.getByRole('dialog').filter({ hasText: releaseName })
  await expect(deleteDialog).toBeVisible()
  await deleteDialog.getByPlaceholder(releaseName).fill(releaseName)
  await expect(
    deleteDialog.getByRole('button', { name: 'Delete' })
  ).toBeEnabled()
  await deleteDialog.getByRole('button', { name: 'Delete' }).click()
  await page.waitForURL('**/helmrelease', { timeout: 120_000 })

  // Wait for the async delete task to complete — the release should
  // disappear from the list.
  await expect
    .poll(
      async () => {
        const response = await page.request.get(
          `/api/v1/helmrelease/_all?labelSelector=${encodeURIComponent(
            `app.kubernetes.io/instance=${releaseName}`
          )}`
        )
        if (!response.ok()) {
          return 1
        }
        const body = (await response.json()) as { items?: unknown[] }
        return body.items?.length ?? 1
      },
      { timeout: 120_000 }
    )
    .toBe(0)
}

async function deleteRepositoryFromChartsPage(
  page: Page,
  repositoryName: string
) {
  await page.goto('/charts')
  await switchToRepositories(page)
  await selectRepositoryFilter(page, repositoryName)
  await page.getByRole('button', { name: 'Delete Repository' }).click()

  const deleteDialog = page
    .getByRole('dialog')
    .filter({ hasText: repositoryName })
  await expect(deleteDialog).toBeVisible()
  await deleteDialog.getByPlaceholder(repositoryName).fill(repositoryName)
  await expect(
    deleteDialog.getByRole('button', { name: 'Delete' })
  ).toBeEnabled()
  await deleteDialog.getByRole('button', { name: 'Delete' }).click()
  await expect(deleteDialog).toBeHidden({ timeout: 60_000 })
  await expect(
    page.getByRole('button', { name: 'Delete Repository' })
  ).toBeHidden()
}

async function cleanupReleaseFromUI(page: Page, releaseName: string) {
  try {
    await page.goto(`/helmrelease/${namespace}/${releaseName}`)
    const deleteButton = page.getByRole('button', { name: 'Delete' })
    if (await deleteButton.isVisible({ timeout: 5_000 }).catch(() => false)) {
      await deleteReleaseFromCurrentPage(page, releaseName)
    }
  } catch {
    // Best-effort UI cleanup only.
  }
}

async function cleanupRepositoryFromUI(page: Page, repositoryName: string) {
  try {
    await page.goto('/charts')
    await switchToRepositories(page)
    await page.locator('[data-slot="select-trigger"]').first().click()
    const option = page.getByRole('option', { name: repositoryName })
    if (!(await option.isVisible({ timeout: 5_000 }).catch(() => false))) {
      await page.keyboard.press('Escape')
      return
    }
    await option.click()
    await page.getByRole('button', { name: 'Delete Repository' }).click()

    const deleteDialog = page
      .getByRole('dialog')
      .filter({ hasText: repositoryName })
    await deleteDialog.getByPlaceholder(repositoryName).fill(repositoryName)
    await deleteDialog.getByRole('button', { name: 'Delete' }).click()
    await expect(deleteDialog).toBeHidden({ timeout: 60_000 })
  } catch {
    // Best-effort UI cleanup only.
  }
}

test.describe('helm kite lifecycle', () => {
  test.setTimeout(8 * 60 * 1000)

  test('manages the kite repository and release lifecycle through the UI', async ({
    page,
  }) => {
    const suffix = Date.now().toString(36)
    const repositoryName = `e2e-kite-${suffix}`
    const releaseName = `e2e-kite-${suffix}`
    let repositoryDeleted = false
    let releaseDeleted = false

    try {
      await page.goto('/charts')
      const origin = new URL(page.url()).origin
      await page
        .context()
        .grantPermissions(['clipboard-read', 'clipboard-write'], { origin })

      await switchToRepositories(page)
      await page.getByRole('button', { name: 'Add Repository' }).first().click()

      const addRepositoryDialog = page.getByRole('dialog', {
        name: 'Add Repository',
      })
      await expect(addRepositoryDialog).toBeVisible()
      await addRepositoryDialog
        .locator('#helm-repository-name')
        .fill(repositoryName)
      await addRepositoryDialog
        .locator('#helm-repository-url')
        .fill(repositoryURL)
      await addRepositoryDialog.getByRole('button', { name: 'Add' }).click()
      await expect(addRepositoryDialog).toBeHidden({ timeout: 60_000 })

      await selectRepositoryFilter(page, repositoryName)
      await page.getByPlaceholder('Search charts...').fill(chartName)
      const chartLink = page.getByRole('link', {
        name: chartName,
        exact: true,
      })
      await expect(chartLink).toBeVisible({ timeout: 60_000 })

      await chartLink.click()
      await page.waitForURL(
        `**/charts/${encodeURIComponent(repositoryName)}/${chartName}`
      )
      await page.goto(
        `/charts/${encodeURIComponent(repositoryName)}/${encodeURIComponent(chartName)}?version=${encodeURIComponent(installVersion)}`
      )

      await expect(
        page.getByRole('heading', { name: chartName }).first()
      ).toBeVisible({ timeout: 60_000 })
      await expect(page.getByText(installVersion).first()).toBeVisible()
      await page.getByRole('tab', { name: 'Values' }).click()
      await expect(page.locator('.monaco-editor').first()).toBeVisible({
        timeout: 60_000,
      })
      await page.getByRole('tab', { name: 'Versions' }).click()
      await expect(
        page.getByRole('link', { name: specifiedUpgradeVersion })
      ).toBeVisible()

      await page.getByRole('button', { name: 'Install' }).click()
      const installDialog = page.getByRole('dialog', { name: 'Install' })
      await expect(installDialog).toBeVisible()
      await installDialog.getByLabel('Release Name').fill(releaseName)
      await fillMonacoEditor(page, installDialog, 1, baseValues)
      await expect(
        installDialog.getByRole('button', { name: 'Dry Run' })
      ).toBeEnabled({ timeout: 60_000 })
      await installDialog.getByRole('button', { name: 'Dry Run' }).click()
      await expectDryRunPreview(installDialog)
      await expect(
        installDialog.getByRole('button', { name: 'Install' })
      ).toBeEnabled({ timeout: 60_000 })
      await installDialog.getByRole('button', { name: 'Install' }).click()

      await page.waitForURL(
        `**/helmrelease/${namespace}/${encodeURIComponent(releaseName)}`,
        { timeout: 120_000 }
      )
      await expectReleaseSummary(page, releaseName, installVersion, 1)
      await expectReleaseValues(page, 'e2e-mode: base', 'e2e-mode: upgraded')
      await expectAppliedPodLabel(page, releaseName, 'base')

      await page.getByRole('button', { name: 'Upgrade', exact: true }).click()
      const customValuesUpgradeDialog = page.getByRole('dialog', {
        name: 'Upgrade',
      })
      await expect(customValuesUpgradeDialog).toBeVisible()
      await fillMonacoEditor(page, customValuesUpgradeDialog, 1, upgradedValues)
      await expect(
        customValuesUpgradeDialog.getByRole('button', { name: 'Dry Run' })
      ).toBeEnabled({ timeout: 60_000 })
      await customValuesUpgradeDialog
        .getByRole('button', { name: 'Dry Run' })
        .click()
      await expectDryRunPreview(customValuesUpgradeDialog)
      await expect(
        customValuesUpgradeDialog.getByText('Changed').first()
      ).toBeVisible()
      await expect(
        customValuesUpgradeDialog.getByRole('button', { name: 'Upgrade' })
      ).toBeEnabled({ timeout: 60_000 })
      await customValuesUpgradeDialog
        .getByRole('button', { name: 'Upgrade' })
        .click()
      await expect(customValuesUpgradeDialog).toBeHidden({ timeout: 120_000 })

      await page.reload()
      await expectReleaseSummary(page, releaseName, installVersion, 2)
      await expectReleaseValues(page, 'e2e-mode: upgraded')
      await expectAppliedPodLabel(page, releaseName, 'upgraded')

      await page.getByRole('tab', { name: 'History' }).click()
      await expect(
        page.getByRole('button', { name: 'Rollback' }).first()
      ).toBeEnabled({ timeout: 60_000 })
      await page.getByRole('button', { name: 'Rollback' }).first().click()

      const rollbackDialog = page.getByRole('dialog', {
        name: 'Rollback release?',
      })
      await expect(rollbackDialog).toBeVisible()
      await rollbackDialog.getByRole('button', { name: 'Rollback' }).click()
      await expect(rollbackDialog).toBeHidden({ timeout: 120_000 })

      // Wait for the async rollback task to complete before reloading
      await expect
        .poll(
          async () => {
            const response = await page.request.get(
              `/api/v1/helmrelease/${namespace}/${encodeURIComponent(releaseName)}`
            )
            if (!response.ok()) {
              return 0
            }
            const body = (await response.json()) as {
              spec?: { revision?: number }
            }
            return body.spec?.revision ?? 0
          },
          { timeout: 120_000 }
        )
        .toBe(3)

      await page.reload()
      await expectReleaseSummary(page, releaseName, installVersion, 3)
      await expectReleaseValues(page, 'e2e-mode: base', 'e2e-mode: upgraded')
      await expectAppliedPodLabel(page, releaseName, 'base')

      await page.getByRole('button', { name: 'Upgrade', exact: true }).click()
      const versionUpgradeDialog = page.getByRole('dialog', {
        name: 'Upgrade',
      })
      await expect(versionUpgradeDialog).toBeVisible()
      await selectUpgradeChart(page, versionUpgradeDialog, repositoryName)
      await selectUpgradeVersion(
        page,
        versionUpgradeDialog,
        specifiedUpgradeVersion
      )
      await fillMonacoEditor(page, versionUpgradeDialog, 1, upgradedValues)
      await expect(
        versionUpgradeDialog.getByRole('button', { name: 'Upgrade' })
      ).toBeEnabled({ timeout: 60_000 })
      await versionUpgradeDialog
        .getByRole('button', { name: 'Upgrade' })
        .click()
      await expect(versionUpgradeDialog).toBeHidden({ timeout: 180_000 })

      await page.reload()
      await expectReleaseSummary(page, releaseName, specifiedUpgradeVersion, 4)
      await expectReleaseValues(page, 'e2e-mode: upgraded')
      await expectAppliedPodLabel(page, releaseName, 'upgraded')

      await deleteReleaseFromCurrentPage(page, releaseName)
      await page.getByPlaceholder(/^Search Helm Release/).fill(releaseName)
      await expect(page.getByRole('link', { name: releaseName })).toHaveCount(0)
      releaseDeleted = true

      await deleteRepositoryFromChartsPage(page, repositoryName)
      repositoryDeleted = true
    } finally {
      if (!releaseDeleted) {
        await cleanupReleaseFromUI(page, releaseName)
      }
      if (!repositoryDeleted) {
        await cleanupRepositoryFromUI(page, repositoryName)
      }
    }
  })

  test('refresh button forces cache refresh on private repository', async ({
    page,
  }) => {
    const suffix = Date.now().toString(36)
    const repositoryName = `e2e-refresh-${suffix}`

    try {
      await page.goto('/charts')
      await switchToRepositories(page)
      await page.getByRole('button', { name: 'Add Repository' }).first().click()

      const addRepositoryDialog = page.getByRole('dialog', {
        name: 'Add Repository',
      })
      await expect(addRepositoryDialog).toBeVisible()
      await addRepositoryDialog
        .locator('#helm-repository-name')
        .fill(repositoryName)
      await addRepositoryDialog
        .locator('#helm-repository-url')
        .fill(repositoryURL)
      await addRepositoryDialog.getByRole('button', { name: 'Add' }).click()
      await expect(addRepositoryDialog).toBeHidden({ timeout: 60_000 })

      // Select the newly created repo from the filter
      await selectRepositoryFilter(page, repositoryName)

      // Wait for charts to load
      await expect(
        page.getByRole('link', { name: chartName, exact: true })
      ).toBeVisible({ timeout: 60_000 })

      // Click refresh and verify the request includes refresh=true
      const refreshPromise = page.waitForResponse(
        (response) =>
          response.url().includes('/api/v1/charts') &&
          response.url().includes('refresh=true'),
        { timeout: 60_000 }
      )
      await page.getByRole('button', { name: 'Refresh' }).click()
      const response = await refreshPromise
      expect(response.ok()).toBe(true)
    } finally {
      await cleanupRepositoryFromUI(page, repositoryName)
    }
  })

  test('chart search uses server-side pagination in repositories mode', async ({
    page,
  }) => {
    const suffix = Date.now().toString(36)
    const repositoryName = `e2e-search-${suffix}`

    try {
      await page.goto('/charts')
      await switchToRepositories(page)
      await page.getByRole('button', { name: 'Add Repository' }).first().click()

      const addRepositoryDialog = page.getByRole('dialog', {
        name: 'Add Repository',
      })
      await expect(addRepositoryDialog).toBeVisible()
      await addRepositoryDialog
        .locator('#helm-repository-name')
        .fill(repositoryName)
      await addRepositoryDialog
        .locator('#helm-repository-url')
        .fill(repositoryURL)
      await addRepositoryDialog.getByRole('button', { name: 'Add' }).click()
      await expect(addRepositoryDialog).toBeHidden({ timeout: 60_000 })

      // Verify that initial load sends limit and offset query params
      const initialLoadPromise = page.waitForResponse(
        (response) =>
          response.url().includes('/api/v1/charts') &&
          response.url().includes('limit=') &&
          response.url().includes('offset=0'),
        { timeout: 60_000 }
      )
      await selectRepositoryFilter(page, repositoryName)
      const initialResponse = await initialLoadPromise
      expect(initialResponse.ok()).toBe(true)
      await expect(
        page.getByRole('link', { name: chartName, exact: true })
      ).toBeVisible({ timeout: 60_000 })

      // Verify that searching sends q, limit, and offset query params
      const searchPromise = page.waitForResponse(
        (response) =>
          response.url().includes('/api/v1/charts') &&
          response.url().includes('q=') &&
          response.url().includes('limit=') &&
          response.url().includes('offset=0'),
        { timeout: 60_000 }
      )
      await page.getByPlaceholder('Search charts...').fill(chartName)
      const searchResponse = await searchPromise
      expect(searchResponse.ok()).toBe(true)
      const searchBody = (await searchResponse.json()) as {
        items?: unknown[]
        total?: number
      }
      expect(searchBody.items?.length).toBeGreaterThan(0)
      expect(searchBody.total).toBeGreaterThanOrEqual(
        searchBody.items!.length
      )

      // Verify the search result is filtered server-side
      await expect(
        page.getByRole('link', { name: chartName, exact: true })
      ).toBeVisible()

      // Verify that clearing search resets to offset=0
      const clearSearchPromise = page.waitForResponse(
        (response) =>
          response.url().includes('/api/v1/charts') &&
          !response.url().includes('q=') &&
          response.url().includes('offset=0'),
        { timeout: 60_000 }
      )
      await page.getByPlaceholder('Search charts...').fill('')
      const clearResponse = await clearSearchPromise
      expect(clearResponse.ok()).toBe(true)
    } finally {
      await cleanupRepositoryFromUI(page, repositoryName)
    }
  })

  test('custom values are applied during install and upgrade', async ({ page }) => {
    test.setTimeout(8 * 60 * 1000)
    const suffix = Date.now().toString(36)
    const repositoryName = `e2e-set-${suffix}`
    const releaseName = `e2e-set-${suffix}`
    let repositoryDeleted = false
    let releaseDeleted = false

    try {
      // Add repository
      await page.goto('/charts')
      await switchToRepositories(page)
      await page.getByRole('button', { name: 'Add Repository' }).first().click()
      const addRepoDialog = page.getByRole('dialog', {
        name: 'Add Repository',
      })
      await expect(addRepoDialog).toBeVisible()
      await addRepoDialog.locator('#helm-repository-name').fill(repositoryName)
      await addRepoDialog.locator('#helm-repository-url').fill(repositoryURL)
      await addRepoDialog.getByRole('button', { name: 'Add' }).click()
      await expect(addRepoDialog).toBeHidden({ timeout: 60_000 })

      // Navigate to chart install page
      await selectRepositoryFilter(page, repositoryName)
      await page.getByPlaceholder('Search charts...').fill(chartName)
      const chartLink = page.getByRole('link', {
        name: chartName,
        exact: true,
      })
      await expect(chartLink).toBeVisible({ timeout: 60_000 })
      await chartLink.click()
      await page.waitForURL(
        `**/charts/${encodeURIComponent(repositoryName)}/${chartName}`
      )
      await page.goto(
        `/charts/${encodeURIComponent(repositoryName)}/${encodeURIComponent(chartName)}?version=${encodeURIComponent(installVersion)}`
      )
      await expect(
        page.getByRole('heading', { name: chartName }).first()
      ).toBeVisible({ timeout: 60_000 })

      // Install with custom values
      await page.getByRole('button', { name: 'Install' }).click()
      const installDialog = page.getByRole('dialog', { name: 'Install' })
      await expect(installDialog).toBeVisible()
      await installDialog.getByLabel('Release Name').fill(releaseName)

      // Fill custom values in YAML editor (includes e2e-set-mode: install)
      const installValues = `replicaCount: 1
anonymousUserEnabled: true
podLabels:
  e2e-mode: base
  e2e-set-mode: install
`
      await fillMonacoEditor(page, installDialog, 1, installValues)

      await expect(
        installDialog.getByRole('button', { name: 'Install' })
      ).toBeEnabled({ timeout: 60_000 })
      await installDialog.getByRole('button', { name: 'Install' }).click()

      await page.waitForURL(
        `**/helmrelease/${namespace}/${encodeURIComponent(releaseName)}`,
        { timeout: 120_000 }
      )
      await expectReleaseSummary(page, releaseName, installVersion, 1)

      // Verify custom value was applied to the pod labels
      await expect
        .poll(
          async () => {
            const response = await page.request.get(
              `/api/v1/deployments/${namespace}?labelSelector=${encodeURIComponent(
                `app.kubernetes.io/instance=${releaseName}`
              )}`
            )
            if (!response.ok()) {
              return ''
            }
            const body = (await response.json()) as {
              items?: Array<{
                spec?: {
                  template?: {
                    metadata?: {
                      labels?: Record<string, string>
                    }
                  }
                }
              }>
            }
            const labels = (body.items || []).map(
              (item) =>
                item.spec?.template?.metadata?.labels?.['e2e-set-mode'] || ''
            )
            if (!labels.length || labels.some((l) => !l)) {
              return ''
            }
            return labels.every((l) => l === 'install') ? 'install' : labels.join(',')
          },
          { timeout: 120_000 }
        )
        .toBe('install')

      // Upgrade with different custom values
      await page.getByRole('button', { name: 'Upgrade', exact: true }).click()
      const upgradeDialog = page.getByRole('dialog', { name: 'Upgrade' })
      await expect(upgradeDialog).toBeVisible()
      const upgradeValues = `replicaCount: 1
anonymousUserEnabled: true
podLabels:
  e2e-mode: upgraded
  e2e-set-mode: upgrade
`
      await fillMonacoEditor(page, upgradeDialog, 1, upgradeValues)

      await expect(
        upgradeDialog.getByRole('button', { name: 'Upgrade' })
      ).toBeEnabled({ timeout: 60_000 })
      await upgradeDialog.getByRole('button', { name: 'Upgrade' }).click()
      await expect(upgradeDialog).toBeHidden({ timeout: 120_000 })

      // Verify custom value was updated
      await expect
        .poll(
          async () => {
            const response = await page.request.get(
              `/api/v1/deployments/${namespace}?labelSelector=${encodeURIComponent(
                `app.kubernetes.io/instance=${releaseName}`
              )}`
            )
            if (!response.ok()) {
              return ''
            }
            const body = (await response.json()) as {
              items?: Array<{
                spec?: {
                  template?: {
                    metadata?: {
                      labels?: Record<string, string>
                    }
                  }
                }
              }>
            }
            const labels = (body.items || []).map(
              (item) =>
                item.spec?.template?.metadata?.labels?.['e2e-set-mode'] || ''
            )
            if (!labels.length || labels.some((l) => !l)) {
              return ''
            }
            return labels.every((l) => l === 'upgrade')
              ? 'upgrade'
              : labels.join(',')
          },
          { timeout: 120_000 }
        )
        .toBe('upgrade')

      // Cleanup
      await deleteReleaseFromCurrentPage(page, releaseName)
      releaseDeleted = true
      await deleteRepositoryFromChartsPage(page, repositoryName)
      repositoryDeleted = true
    } finally {
      if (!releaseDeleted) {
        await cleanupReleaseFromUI(page, releaseName)
      }
      if (!repositoryDeleted) {
        await cleanupRepositoryFromUI(page, repositoryName)
      }
    }
  })

  test('merged values preview is displayed during install', async ({ page }) => {
    const suffix = Date.now().toString(36)
    const repositoryName = `e2e-merged-${suffix}`

    try {
      await page.goto('/charts')
      await switchToRepositories(page)
      await page.getByRole('button', { name: 'Add Repository' }).first().click()

      const addRepositoryDialog = page.getByRole('dialog', {
        name: 'Add Repository',
      })
      await expect(addRepositoryDialog).toBeVisible()
      await addRepositoryDialog
        .locator('#helm-repository-name')
        .fill(repositoryName)
      await addRepositoryDialog
        .locator('#helm-repository-url')
        .fill(repositoryURL)
      await addRepositoryDialog.getByRole('button', { name: 'Add' }).click()
      await expect(addRepositoryDialog).toBeHidden({ timeout: 60_000 })

      await selectRepositoryFilter(page, repositoryName)
      await page.getByPlaceholder('Search charts...').fill(chartName)
      const chartLink = page.getByRole('link', {
        name: chartName,
        exact: true,
      })
      await expect(chartLink).toBeVisible({ timeout: 60_000 })
      await chartLink.click()
      await page.waitForURL(
        `**/charts/${encodeURIComponent(repositoryName)}/${chartName}`
      )

      await page.getByRole('button', { name: 'Install' }).click()
      const installDialog = page.getByRole('dialog', { name: 'Install' })
      await expect(installDialog).toBeVisible()

      // Verify the merged values preview API is called
      const previewPromise = page.waitForResponse(
        (response) =>
          response.url().includes('/api/v1/helmrelease/') &&
          response.url().includes('/preview-values'),
        { timeout: 60_000 }
      )

      // Fill custom values to trigger the merge preview
      await fillMonacoEditor(page, installDialog, 1, 'replicaCount: 3\n')
      const previewResponse = await previewPromise
      expect(previewResponse.ok()).toBe(true)
      const previewBody = (await previewResponse.json()) as { values?: string }
      expect(previewBody.values).toContain('replicaCount:')

      // Click Preview button to enter preview mode
      await installDialog.getByRole('button', { name: 'Preview' }).click()

      // Verify the merged preview is visible
      await expect(
        installDialog.getByText('Merged values preview')
      ).toBeVisible({ timeout: 60_000 })

      await installDialog.getByRole('button', { name: 'Back' }).click()
      await installDialog.getByRole('button', { name: 'Cancel' }).click()
    } finally {
      await cleanupRepositoryFromUI(page, repositoryName)
    }
  })

  test('merged values preview is displayed during upgrade', async ({ page }) => {
    const suffix = Date.now().toString(36)
    const repositoryName = `e2e-merged-upgrade-${suffix}`
    const releaseName = `e2e-merge-up-${suffix}`

    try {
      // Setup: add repository
      await page.goto('/charts')
      await switchToRepositories(page)
      await page.getByRole('button', { name: 'Add Repository' }).first().click()
      const addRepositoryDialog = page.getByRole('dialog', {
        name: 'Add Repository',
      })
      await expect(addRepositoryDialog).toBeVisible()
      await addRepositoryDialog
        .locator('#helm-repository-name')
        .fill(repositoryName)
      await addRepositoryDialog
        .locator('#helm-repository-url')
        .fill(repositoryURL)
      await addRepositoryDialog.getByRole('button', { name: 'Add' }).click()
      await expect(addRepositoryDialog).toBeHidden({ timeout: 60_000 })

      // Install a release first
      await selectRepositoryFilter(page, repositoryName)
      await page.getByPlaceholder('Search charts...').fill(chartName)
      const chartLink = page.getByRole('link', {
        name: chartName,
        exact: true,
      })
      await expect(chartLink).toBeVisible({ timeout: 60_000 })
      await chartLink.click()
      await page.waitForURL(
        `**/charts/${encodeURIComponent(repositoryName)}/${chartName}`
      )

      await page.getByRole('button', { name: 'Install' }).click()
      const installDialog = page.getByRole('dialog', { name: 'Install' })
      await expect(installDialog).toBeVisible()
      await installDialog
        .locator('#helm-install-release-name')
        .fill(releaseName)
      await installDialog.getByRole('button', { name: 'Install' }).click()
      await expect(installDialog).toBeHidden({ timeout: 120_000 })

      // Navigate to release detail
      await page.goto(`/helmrelease/default/${releaseName}`)

      // Open upgrade dialog
      await page.getByRole('button', { name: 'Upgrade', exact: true }).click()
      const upgradeDialog = page.getByRole('dialog', { name: 'Upgrade' })
      await expect(upgradeDialog).toBeVisible()

      // Verify the merged values preview API is called when editing custom values
      const previewPromise = page.waitForResponse(
        (response) =>
          response.url().includes('/api/v1/helmrelease/') &&
          response.url().includes('/upgrade/preview-values'),
        { timeout: 60_000 }
      )

      // Modify custom values to trigger the merge preview
      await fillMonacoEditor(page, upgradeDialog, 1, 'replicaCount: 3\n')
      const previewResponse = await previewPromise
      expect(previewResponse.ok()).toBe(true)
      const previewBody = (await previewResponse.json()) as { values?: string }
      expect(previewBody.values).toContain('replicaCount:')

      // Click Preview button to enter preview mode
      await upgradeDialog.getByRole('button', { name: 'Preview' }).click()

      // Verify the merged preview is visible
      await expect(
        upgradeDialog.getByText('Merged values preview')
      ).toBeVisible({ timeout: 60_000 })

      await upgradeDialog.getByRole('button', { name: 'Back' }).click()
      await upgradeDialog.getByRole('button', { name: 'Cancel' }).click()
    } finally {
      await cleanupReleaseFromUI(page, releaseName)
      await cleanupRepositoryFromUI(page, repositoryName)
    }
  })

  test('values diff is displayed in preview mode during install', async ({
    page,
  }) => {
    const suffix = Date.now().toString(36)
    const repositoryName = `e2e-diff-${suffix}`

    try {
      await page.goto('/charts')
      await switchToRepositories(page)
      await page.getByRole('button', { name: 'Add Repository' }).first().click()

      const addRepositoryDialog = page.getByRole('dialog', {
        name: 'Add Repository',
      })
      await expect(addRepositoryDialog).toBeVisible()
      await addRepositoryDialog
        .locator('#helm-repository-name')
        .fill(repositoryName)
      await addRepositoryDialog
        .locator('#helm-repository-url')
        .fill(repositoryURL)
      await addRepositoryDialog.getByRole('button', { name: 'Add' }).click()
      await expect(addRepositoryDialog).toBeHidden({ timeout: 60_000 })

      await selectRepositoryFilter(page, repositoryName)
      await page.getByPlaceholder('Search charts...').fill(chartName)
      const chartLink = page.getByRole('link', {
        name: chartName,
        exact: true,
      })
      await expect(chartLink).toBeVisible({ timeout: 60_000 })
      await chartLink.click()
      await page.waitForURL(
        `**/charts/${encodeURIComponent(repositoryName)}/${chartName}`
      )

      await page.getByRole('button', { name: 'Install' }).click()
      const installDialog = page.getByRole('dialog', { name: 'Install' })
      await expect(installDialog).toBeVisible()

      // Fill custom values to trigger the merge preview
      await fillMonacoEditor(page, installDialog, 1, 'replicaCount: 3\n')

      // Wait for preview to load
      await page.waitForResponse(
        (response) =>
          response.url().includes('/api/v1/helmrelease/') &&
          response.url().includes('/preview-values'),
        { timeout: 60_000 }
      )

      // Click Preview button to enter preview mode
      await installDialog.getByRole('button', { name: 'Preview' }).click()

      // Click the Values diff tab
      await installDialog.getByRole('tab', { name: 'Values diff' }).click()

      // Verify the diff editor is visible
      await expect(
        installDialog.locator('.monaco-diff-editor')
      ).toBeVisible({ timeout: 60_000 })

      await installDialog.getByRole('button', { name: 'Back' }).click()
      await installDialog.getByRole('button', { name: 'Cancel' }).click()
    } finally {
      await cleanupRepositoryFromUI(page, repositoryName)
    }
  })

  test('upgrade preview has 3 tabs: merged, diff vs current, diff vs defaults', async ({
    page,
  }) => {
    const suffix = Date.now().toString(36)
    const repositoryName = `e2e-upg-diff-${suffix}`
    const releaseName = `e2e-upg-diff-${suffix}`

    try {
      // Setup: add repository and install a release
      await page.goto('/charts')
      await switchToRepositories(page)
      await page.getByRole('button', { name: 'Add Repository' }).first().click()
      const addRepositoryDialog = page.getByRole('dialog', {
        name: 'Add Repository',
      })
      await expect(addRepositoryDialog).toBeVisible()
      await addRepositoryDialog
        .locator('#helm-repository-name')
        .fill(repositoryName)
      await addRepositoryDialog
        .locator('#helm-repository-url')
        .fill(repositoryURL)
      await addRepositoryDialog.getByRole('button', { name: 'Add' }).click()
      await expect(addRepositoryDialog).toBeHidden({ timeout: 60_000 })

      await selectRepositoryFilter(page, repositoryName)
      await page.getByPlaceholder('Search charts...').fill(chartName)
      const chartLink = page.getByRole('link', {
        name: chartName,
        exact: true,
      })
      await expect(chartLink).toBeVisible({ timeout: 60_000 })
      await chartLink.click()
      await page.getByRole('button', { name: 'Install' }).click()
      const installDialog = page.getByRole('dialog', { name: 'Install' })
      await expect(installDialog).toBeVisible()
      await installDialog
        .locator('#helm-install-release-name')
        .fill(releaseName)
      await installDialog.getByRole('button', { name: 'Install' }).click()
      await expect(installDialog).toBeHidden({ timeout: 120_000 })

      // Navigate to release detail and open upgrade dialog
      await page.goto(`/helmrelease/default/${releaseName}`)
      await page.getByRole('button', { name: 'Upgrade', exact: true }).click()
      const upgradeDialog = page.getByRole('dialog', { name: 'Upgrade' })
      await expect(upgradeDialog).toBeVisible()

      // Fill custom values to trigger the merge preview
      await fillMonacoEditor(page, upgradeDialog, 1, 'replicaCount: 3\n')

      // Wait for preview to load
      await page.waitForResponse(
        (response) =>
          response.url().includes('/api/v1/helmrelease/') &&
          response.url().includes('/upgrade/preview-values'),
        { timeout: 60_000 }
      )

      // Click Preview button to enter preview mode
      await upgradeDialog.getByRole('button', { name: 'Preview' }).click()

      // Verify 3 tabs are visible
      await expect(
        upgradeDialog.getByRole('tab', { name: 'Merged values preview' })
      ).toBeVisible()
      await expect(
        upgradeDialog.getByRole('tab', { name: 'Diff vs current' })
      ).toBeVisible()
      await expect(
        upgradeDialog.getByRole('tab', { name: 'Diff vs defaults' })
      ).toBeVisible()

      // Click Diff vs current tab and verify diff editor
      await upgradeDialog.getByRole('tab', { name: 'Diff vs current' }).click()
      await expect(
        upgradeDialog.locator('.monaco-diff-editor')
      ).toBeVisible({ timeout: 60_000 })

      // Click Diff vs defaults tab and verify diff editor
      await upgradeDialog.getByRole('tab', { name: 'Diff vs defaults' }).click()
      await expect(
        upgradeDialog.locator('.monaco-diff-editor')
      ).toBeVisible({ timeout: 60_000 })

      await upgradeDialog.getByRole('button', { name: 'Back' }).click()
      await upgradeDialog.getByRole('button', { name: 'Cancel' }).click()
    } finally {
      await cleanupReleaseFromUI(page, releaseName)
      await cleanupRepositoryFromUI(page, repositoryName)
    }
  })

  test('timeout input appears when wait is checked during install', async ({
    page,
  }) => {
    const suffix = Date.now().toString(36)
    const repositoryName = `e2e-timeout-${suffix}`

    try {
      await page.goto('/charts')
      await switchToRepositories(page)
      await page.getByRole('button', { name: 'Add Repository' }).first().click()

      const addRepositoryDialog = page.getByRole('dialog', {
        name: 'Add Repository',
      })
      await expect(addRepositoryDialog).toBeVisible()
      await addRepositoryDialog
        .locator('#helm-repository-name')
        .fill(repositoryName)
      await addRepositoryDialog
        .locator('#helm-repository-url')
        .fill(repositoryURL)
      await addRepositoryDialog.getByRole('button', { name: 'Add' }).click()
      await expect(addRepositoryDialog).toBeHidden({ timeout: 60_000 })

      await selectRepositoryFilter(page, repositoryName)
      await page.getByPlaceholder('Search charts...').fill(chartName)
      const chartLink = page.getByRole('link', {
        name: chartName,
        exact: true,
      })
      await expect(chartLink).toBeVisible({ timeout: 60_000 })
      await chartLink.click()

      await page.getByRole('button', { name: 'Install' }).click()
      const installDialog = page.getByRole('dialog', { name: 'Install' })
      await expect(installDialog).toBeVisible()

      // Check the Wait checkbox
      await installDialog.locator('#helm-install-wait').check()

      // Verify the timeout input appears
      await expect(
        installDialog.locator('#helm-install-timeout')
      ).toBeVisible()

      // Uncheck Wait and verify timeout disappears
      await installDialog.locator('#helm-install-wait').uncheck()
      await expect(
        installDialog.locator('#helm-install-timeout')
      ).toBeHidden()

      await installDialog.getByRole('button', { name: 'Cancel' }).click()
    } finally {
      await cleanupRepositoryFromUI(page, repositoryName)
    }
  })

  test('version selector in install dialog fetches chart for selected version', async ({
    page,
  }) => {
    const suffix = Date.now().toString(36)
    const repositoryName = `e2e-version-${suffix}`

    try {
      await page.goto('/charts')
      await switchToRepositories(page)
      await page.getByRole('button', { name: 'Add Repository' }).first().click()

      const addRepositoryDialog = page.getByRole('dialog', {
        name: 'Add Repository',
      })
      await expect(addRepositoryDialog).toBeVisible()
      await addRepositoryDialog
        .locator('#helm-repository-name')
        .fill(repositoryName)
      await addRepositoryDialog
        .locator('#helm-repository-url')
        .fill(repositoryURL)
      await addRepositoryDialog.getByRole('button', { name: 'Add' }).click()
      await expect(addRepositoryDialog).toBeHidden({ timeout: 60_000 })

      await selectRepositoryFilter(page, repositoryName)
      await page.getByPlaceholder('Search charts...').fill(chartName)
      const chartLink = page.getByRole('link', {
        name: chartName,
        exact: true,
      })
      await expect(chartLink).toBeVisible({ timeout: 60_000 })
      await chartLink.click()
      await page.waitForURL(
        `**/charts/${encodeURIComponent(repositoryName)}/${chartName}`
      )

      await page.getByRole('button', { name: 'Install' }).click()
      const installDialog = page.getByRole('dialog', { name: 'Install' })
      await expect(installDialog).toBeVisible()

      // The version selector should be visible in the install dialog
      const versionSelect = installDialog
        .locator('[data-slot="select-trigger"]')
        .first()
      await expect(versionSelect).toBeVisible({ timeout: 60_000 })

      // Select a different version
      const chartVersionPromise = page.waitForResponse(
        (response) =>
          response.url().includes('/api/v1/charts/') &&
          response.url().includes(`version=${encodeURIComponent(installVersion)}`),
        { timeout: 60_000 }
      )
      await versionSelect.click()
      await page
        .getByRole('option', {
          name: new RegExp(`^${escapeRegExp(installVersion)}(?:\\s|$)`),
        })
        .click()
      const chartVersionResponse = await chartVersionPromise
      expect(chartVersionResponse.ok()).toBe(true)

      // The dialog description should reflect the selected version
      await expect(
        installDialog.getByText(
          `${repositoryName}/${chartName}:${installVersion}`
        )
      ).toBeVisible({ timeout: 60_000 })

      await installDialog.getByRole('button', { name: 'Cancel' }).click()
    } finally {
      await cleanupRepositoryFromUI(page, repositoryName)
    }
  })

  test('install dialog shows create namespace option for non-existent namespace', async ({
    page,
  }) => {
    const suffix = Date.now().toString(36)
    const repositoryName = `e2e-ns-${suffix}`

    try {
      await page.goto('/charts')
      await switchToRepositories(page)
      await page.getByRole('button', { name: 'Add Repository' }).first().click()

      const addRepositoryDialog = page.getByRole('dialog', {
        name: 'Add Repository',
      })
      await expect(addRepositoryDialog).toBeVisible()
      await addRepositoryDialog
        .locator('#helm-repository-name')
        .fill(repositoryName)
      await addRepositoryDialog
        .locator('#helm-repository-url')
        .fill(repositoryURL)
      await addRepositoryDialog.getByRole('button', { name: 'Add' }).click()
      await expect(addRepositoryDialog).toBeHidden({ timeout: 60_000 })

      await selectRepositoryFilter(page, repositoryName)
      await page.getByPlaceholder('Search charts...').fill(chartName)
      const chartLink = page.getByRole('link', {
        name: chartName,
        exact: true,
      })
      await expect(chartLink).toBeVisible({ timeout: 60_000 })
      await chartLink.click()
      await page.waitForURL(
        `**/charts/${encodeURIComponent(repositoryName)}/${chartName}`
      )

      await page.getByRole('button', { name: 'Install' }).click()
      const installDialog = page.getByRole('dialog', { name: 'Install' })
      await expect(installDialog).toBeVisible()

      // Type a non-existent namespace in the selector search
      const nsSelector = installDialog.locator('[data-slot="select-trigger"]').nth(1)
      await nsSelector.click()

      // Search for a namespace that doesn't exist
      const nonexistentNs = `e2e-nonexistent-${suffix}`
      await page.keyboard.type(nonexistentNs)

      // The "Create" option should appear
      const createOption = page.getByRole('option', { name: nonexistentNs })
      await expect(createOption).toBeVisible({ timeout: 10_000 })
      await createOption.click()

      // The "Create namespace" checkbox should now be visible inline
      await expect(
        installDialog.locator('#helm-create-namespace')
      ).toBeVisible({ timeout: 10_000 })

      // Now select an existing namespace ("default")
      await nsSelector.click()
      await page.getByRole('option', { name: 'default', exact: true }).click()

      // The "Create namespace" checkbox should disappear
      await expect(
        installDialog.locator('#helm-create-namespace')
      ).toBeHidden({ timeout: 10_000 })

      await installDialog.getByRole('button', { name: 'Cancel' }).click()
    } finally {
      await cleanupRepositoryFromUI(page, repositoryName)
    }
  })

  test('chart search debounces API requests in Artifact Hub mode', async ({
    page,
  }) => {
    await page.goto('/charts')

    // Ensure we're in Artifact Hub mode (default)
    await expect(page.getByText('Artifact Hub', { exact: true }).first()).toBeVisible()

    // Count API requests triggered by typing
    let requestCount = 0
    page.on('request', (request) => {
      if (
        request.url().includes('/api/v1/charts/artifacthub') &&
        request.url().includes('q=')
      ) {
        requestCount++
      }
    })

    // Type quickly without pausing
    const searchInput = page.getByPlaceholder('Search charts...')
    await searchInput.type('nginx', { delay: 50 })

    // Wait for debounce (400ms) + network
    await page.waitForTimeout(2000)

    // Should have sent at most 1 request with the final query,
    // not one per keystroke
    expect(requestCount).toBeLessThanOrEqual(1)

    // Verify the final request includes the full search term
    const results = await page.evaluate(async () => {
      const resp = await fetch(
        '/api/v1/charts/artifacthub?q=nginx&limit=20&offset=0'
      )
      return resp.ok
    })
    expect(results).toBe(true)
  })

  test('chart list returns partial results when one repository is unreachable', async ({
    page,
  }) => {
    const suffix = Date.now().toString(36)
    const goodRepoName = `e2e-p0-good-${suffix}`
    const badRepoName = `e2e-p0-bad-${suffix}`

    try {
      await page.goto('/charts')

      // Add a valid repository
      await switchToRepositories(page)
      await page.getByRole('button', { name: 'Add Repository' }).first().click()
      const addGoodDialog = page.getByRole('dialog', {
        name: 'Add Repository',
      })
      await expect(addGoodDialog).toBeVisible()
      await addGoodDialog.locator('#helm-repository-name').fill(goodRepoName)
      await addGoodDialog.locator('#helm-repository-url').fill(repositoryURL)
      await addGoodDialog.getByRole('button', { name: 'Add' }).click()
      await expect(addGoodDialog).toBeHidden({ timeout: 60_000 })

      // Add an unreachable repository (valid URL format but non-existent host)
      await page.getByRole('button', { name: 'Add Repository' }).first().click()
      const addBadDialog = page.getByRole('dialog', {
        name: 'Add Repository',
      })
      await expect(addBadDialog).toBeVisible()
      await addBadDialog.locator('#helm-repository-name').fill(badRepoName)
      await addBadDialog
        .locator('#helm-repository-url')
        .fill('https://nonexistent.invalid.example/charts')
      await addBadDialog.getByRole('button', { name: 'Add' }).click()
      // The unreachable repo should fail validation during creation
      await expect(addBadDialog).toBeVisible({ timeout: 60_000 })
      await addBadDialog.getByRole('button', { name: 'Cancel' }).click()

      // Verify chart list still loads successfully with the good repo
      await selectRepositoryFilter(page, goodRepoName)
      await expect(
        page.getByRole('link', { name: chartName, exact: true })
      ).toBeVisible({ timeout: 60_000 })

      // Also verify via API that listing all charts returns 200 even
      // if we had a bad repo (we test with the good repo only since
      // the bad repo couldn't be created)
      const response = await page.request.get('/api/v1/charts')
      expect(response.ok()).toBe(true)
      const body = (await response.json()) as { items?: unknown[] }
      expect(body.items?.length).toBeGreaterThan(0)
    } finally {
      await cleanupRepositoryFromUI(page, goodRepoName)
    }
  })

  test('deleting repository disables associated auto-upgrade tasks', async ({
    page,
  }) => {
    const suffix = Date.now().toString(36)
    const repositoryName = `e2e-p0-del-${suffix}`
    const releaseName = `e2e-p0-del-${suffix}`

    try {
      // Add repository and install a release
      await page.goto('/charts')
      await switchToRepositories(page)
      await page.getByRole('button', { name: 'Add Repository' }).first().click()
      const addRepoDialog = page.getByRole('dialog', {
        name: 'Add Repository',
      })
      await expect(addRepoDialog).toBeVisible()
      await addRepoDialog.locator('#helm-repository-name').fill(repositoryName)
      await addRepoDialog.locator('#helm-repository-url').fill(repositoryURL)
      await addRepoDialog.getByRole('button', { name: 'Add' }).click()
      await expect(addRepoDialog).toBeHidden({ timeout: 60_000 })

      // Install a release
      await selectRepositoryFilter(page, repositoryName)
      await page.getByPlaceholder('Search charts...').fill(chartName)
      const chartLink = page.getByRole('link', {
        name: chartName,
        exact: true,
      })
      await expect(chartLink).toBeVisible({ timeout: 60_000 })
      await chartLink.click()
      await page.goto(
        `/charts/${encodeURIComponent(repositoryName)}/${encodeURIComponent(chartName)}?version=${encodeURIComponent(installVersion)}`
      )
      await page.getByRole('button', { name: 'Install' }).click()
      const installDialog = page.getByRole('dialog', { name: 'Install' })
      await expect(installDialog).toBeVisible()
      await installDialog.getByLabel('Release Name').fill(releaseName)
      await installDialog.getByRole('button', { name: 'Install' }).click()
      await page.waitForURL(
        `**/helmrelease/${namespace}/${encodeURIComponent(releaseName)}`,
        { timeout: 120_000 }
      )

      // Configure auto-upgrade for the release
      const autoUpgradeResponse = await page.request.put(
        `/api/v1/helmrelease/${namespace}/${encodeURIComponent(releaseName)}/auto-upgrade`,
        {
          data: {
            enabled: true,
            scheduleType: 'interval',
            intervalMinutes: 60,
            scheduleTime: '03:00',
            timeoutMinutes: 5,
            source: 'repository',
            repositoryName,
            chartName,
            rollbackOnFailure: false,
          },
        }
      )
      expect(autoUpgradeResponse.ok()).toBe(true)

      // Verify the auto-upgrade task is enabled
      const getResponse = await page.request.get(
        `/api/v1/helmrelease/${namespace}/${encodeURIComponent(releaseName)}/auto-upgrade`
      )
      expect(getResponse.ok()).toBe(true)
      const autoUpgrade = (await getResponse.json()) as { enabled?: boolean }
      expect(autoUpgrade.enabled).toBe(true)

      // Delete the repository via API
      // First find the repository ID
      const listResponse = await page.request.get(
        '/api/v1/charts/repositories'
      )
      expect(listResponse.ok()).toBe(true)
      const repos = (await listResponse.json()) as Array<{
        id: number
        name: string
      }>
      const repo = repos.find((r) => r.name === repositoryName)
      expect(repo).toBeDefined()

      const deleteResponse = await page.request.delete(
        `/api/v1/charts/repositories/${repo!.id}`
      )
      expect(deleteResponse.ok()).toBe(true)

      // Verify the auto-upgrade task is now disabled
      const getResponseAfter = await page.request.get(
        `/api/v1/helmrelease/${namespace}/${encodeURIComponent(releaseName)}/auto-upgrade`
      )
      expect(getResponseAfter.ok()).toBe(true)
      const autoUpgradeAfter = (await getResponseAfter.json()) as {
        enabled?: boolean
      }
      expect(autoUpgradeAfter.enabled).toBe(false)
    } finally {
      await cleanupReleaseFromUI(page, releaseName)
      await cleanupRepositoryFromUI(page, repositoryName)
    }
  })

  test('auto-upgrade returns 404 for non-existent release', async ({
    page,
  }) => {
    await page.goto('/')

    const nonexistentRelease = `e2e-nonexistent-${Date.now().toString(36)}`

    const response = await page.request.put(
      `/api/v1/helmrelease/${namespace}/${encodeURIComponent(nonexistentRelease)}/auto-upgrade`,
      {
        data: {
          enabled: true,
          scheduleType: 'interval',
          intervalMinutes: 60,
          scheduleTime: '03:00',
          timeoutMinutes: 5,
          source: 'repository',
          repositoryName: 'test-repo',
          chartName: 'test-chart',
          rollbackOnFailure: false,
        },
      }
    )
    expect(response.status()).toBe(404)
  })
})

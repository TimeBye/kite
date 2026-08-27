import { expect, test } from '@playwright/test'

import { kindClusterName } from '../env'

const controlPlaneNodeName = `${kindClusterName}-control-plane`

// Use shorter per-test timeout to avoid exceeding the 30-min CI limit
test.describe('YAML download', () => {
  test.use({ timeout: 60_000 })
  test('downloads a single cluster-scoped resource as raw YAML from the detail page', async ({
    page,
  }) => {
    // Register route before navigation to avoid race conditions
    const yamlContent = `apiVersion: v1\nkind: Node\nmetadata:\n  name: ${controlPlaneNodeName}\n`
    await page.route(
      `**/api/v1/nodes/_all/${controlPlaneNodeName}/download*`,
      async (route) => {
        expect(route.request().method()).toBe('GET')
        const url = new URL(route.request().url())
        expect(url.searchParams.get('neat')).toBe('false')
        await route.fulfill({
          status: 200,
          contentType: 'application/yaml',
          headers: {
            'Content-Disposition': `attachment; filename="Node-${controlPlaneNodeName}.yaml"`,
          },
          body: yamlContent,
        })
      }
    )

    await page.goto('/nodes')

    // Navigate to the control-plane node detail page
    const nodeLink = page.getByRole('link', { name: controlPlaneNodeName })
    await expect(nodeLink).toBeVisible()
    await nodeLink.click()
    await page.waitForURL(`**/nodes/${controlPlaneNodeName}`)

    // Click the Download button and select "Raw YAML"
    const downloadButton = page.getByRole('button', { name: 'Download' })
    await expect(downloadButton).toBeVisible()
    await downloadButton.click()
    const download = page.waitForEvent('download')
    await page.getByRole('menuitem', { name: 'Raw YAML' }).click()
    expect((await download).suggestedFilename()).toBe(
      `Node-${controlPlaneNodeName}.yaml`
    )
  })

  test('downloads a single cluster-scoped resource as neat YAML from the detail page', async ({
    page,
  }) => {
    const yamlContent = `apiVersion: v1\nkind: Node\nmetadata:\n  name: ${controlPlaneNodeName}\n`
    await page.route(
      `**/api/v1/nodes/_all/${controlPlaneNodeName}/download*`,
      async (route) => {
        expect(route.request().method()).toBe('GET')
        const url = new URL(route.request().url())
        expect(url.searchParams.get('neat')).toBe('true')
        await route.fulfill({
          status: 200,
          contentType: 'application/yaml',
          headers: {
            'Content-Disposition': `attachment; filename="Node-${controlPlaneNodeName}.yaml"`,
          },
          body: yamlContent,
        })
      }
    )

    await page.goto('/nodes')

    const nodeLink = page.getByRole('link', { name: controlPlaneNodeName })
    await expect(nodeLink).toBeVisible()
    await nodeLink.click()
    await page.waitForURL(`**/nodes/${controlPlaneNodeName}`)

    const downloadButton = page.getByRole('button', { name: 'Download' })
    await expect(downloadButton).toBeVisible()
    await downloadButton.click()
    const download = page.waitForEvent('download')
    await page.getByRole('menuitem', { name: 'Neat YAML' }).click()
    expect((await download).suggestedFilename()).toBe(
      `Node-${controlPlaneNodeName}.yaml`
    )
  })

  test('downloads a single namespace-scoped resource as raw YAML from the detail page', async ({
    page,
  }) => {
    const yamlContent = `apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: kube-root-ca.crt\n  namespace: default\n`
    await page.route(
      '**/api/v1/configmaps/default/kube-root-ca.crt/download*',
      async (route) => {
        expect(route.request().method()).toBe('GET')
        const url = new URL(route.request().url())
        expect(url.searchParams.get('neat')).toBe('false')
        await route.fulfill({
          status: 200,
          contentType: 'application/yaml',
          headers: {
            'Content-Disposition':
              'attachment; filename="ConfigMap-default-kube-root-ca.crt.yaml"',
          },
          body: yamlContent,
        })
      }
    )

    await page.goto('/configmaps')

    // Navigate to the kube-root-ca.crt configmap detail page
    const cmLink = page.getByRole('link', { name: 'kube-root-ca.crt' })
    await expect(cmLink).toBeVisible()
    await cmLink.click()
    await page.waitForURL('**/configmaps/default/kube-root-ca.crt')

    const downloadButton = page.getByRole('button', { name: 'Download' })
    await expect(downloadButton).toBeVisible()
    await downloadButton.click()
    const download = page.waitForEvent('download')
    await page.getByRole('menuitem', { name: 'Raw YAML' }).click()
    expect((await download).suggestedFilename()).toBe(
      'ConfigMap-default-kube-root-ca.crt.yaml'
    )
  })

  test('downloads multiple cluster-scoped resources as a ZIP from the list page', async ({
    page,
  }) => {
    // Register route before navigation
    await page.route('**/api/v1/namespaces/_all/download*', async (route) => {
      expect(route.request().method()).toBe('POST')
      const url = new URL(route.request().url())
      expect(url.searchParams.get('neat')).toBe('false')
      await route.fulfill({
        status: 200,
        contentType: 'application/zip',
        headers: {
          'Content-Disposition': 'attachment; filename="namespaces-test.zip"',
        },
        body: 'PK\x03\x04',
      })
    })

    await page.goto('/namespaces')

    // Select multiple namespaces via row checkboxes
    const kubeSystemRow = page
      .getByRole('row')
      .filter({ hasText: 'kube-system' })
    const kubePublicRow = page
      .getByRole('row')
      .filter({ hasText: 'kube-public' })

    await kubeSystemRow.getByRole('checkbox').click()
    await kubePublicRow.getByRole('checkbox').click()

    // Click the download button in the batch actions area
    const downloadButton = page.getByRole('button', {
      name: /Download.*\(\d+\)/,
    })
    await expect(downloadButton).toBeVisible()
    await downloadButton.click()

    const download = page.waitForEvent('download')
    await page.getByRole('menuitem', { name: 'Raw YAML' }).click()
    const downloadedFile = await download
    expect(downloadedFile.suggestedFilename()).toBe('namespaces-test.zip')
  })

  test('downloads multiple namespace-scoped resources as a ZIP from the list page', async ({
    page,
  }) => {
    // Register route before navigation
    await page.route('**/api/v1/configmaps/download*', async (route) => {
      expect(route.request().method()).toBe('POST')
      const url = new URL(route.request().url())
      expect(url.searchParams.get('neat')).toBe('true')
      await route.fulfill({
        status: 200,
        contentType: 'application/zip',
        headers: {
          'Content-Disposition': 'attachment; filename="configmaps-test.zip"',
        },
        body: 'PK\x03\x04',
      })
    })

    await page.goto('/configmaps')

    // Select multiple configmaps via row checkboxes
    const firstRow = page.getByRole('row').nth(1)
    const secondRow = page.getByRole('row').nth(2)

    await firstRow.getByRole('checkbox').click()
    await secondRow.getByRole('checkbox').click()

    // Click the download button in the batch actions area
    const downloadButton = page.getByRole('button', {
      name: /Download.*\(\d+\)/,
    })
    await expect(downloadButton).toBeVisible()
    await downloadButton.click()

    const download = page.waitForEvent('download')
    await page.getByRole('menuitem', { name: 'Neat YAML' }).click()
    const downloadedFile = await download
    expect(downloadedFile.suggestedFilename()).toBe('configmaps-test.zip')
  })
})

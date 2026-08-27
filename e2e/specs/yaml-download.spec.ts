import { expect, test } from '@playwright/test'

import { kindClusterName } from '../env'

const controlPlaneNodeName = `${kindClusterName}-control-plane`

test.describe('YAML download', () => {
  test('downloads a single resource as raw YAML from the detail page', async ({
    page,
  }) => {
    await page.goto('/nodes')

    // Navigate to the control-plane node detail page
    const nodeLink = page.getByRole('link', { name: controlPlaneNodeName })
    await expect(nodeLink).toBeVisible()
    await nodeLink.click()
    await page.waitForURL(`**/nodes/${controlPlaneNodeName}`)

    // Intercept the download API call
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

  test('downloads a single resource as neat YAML from the detail page', async ({
    page,
  }) => {
    await page.goto('/nodes')

    const nodeLink = page.getByRole('link', { name: controlPlaneNodeName })
    await expect(nodeLink).toBeVisible()
    await nodeLink.click()
    await page.waitForURL(`**/nodes/${controlPlaneNodeName}`)

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

    const downloadButton = page.getByRole('button', { name: 'Download' })
    await expect(downloadButton).toBeVisible()
    await downloadButton.click()
    const download = page.waitForEvent('download')
    await page.getByRole('menuitem', { name: 'Neat YAML' }).click()
    expect((await download).suggestedFilename()).toBe(
      `Node-${controlPlaneNodeName}.yaml`
    )
  })

  test('downloads multiple resources as a ZIP from the list page', async ({
    page,
  }) => {
    await page.goto('/namespaces')

    // Intercept the batch download API call
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
})

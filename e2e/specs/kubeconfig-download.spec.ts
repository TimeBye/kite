import { expect, test } from '@playwright/test'

test('downloads a kubeconfig for the current cluster with the selected TTL', async ({
  page,
}) => {
  await page.route('**/api/v1/kubeconfig', async (route) => {
    expect(route.request().method()).toBe('POST')
    const { clusterUUIDs, ttlSeconds } = await route.request().postDataJSON()
    expect(ttlSeconds).toBe(86400)
    expect(clusterUUIDs[0]).toEqual(expect.any(String))
    expect(clusterUUIDs[0]).not.toBe('')
    await route.fulfill({
      status: 200,
      contentType: 'application/yaml',
      headers: {
        'Content-Disposition': 'attachment; filename="kite-kubeconfig.yaml"',
        'Cache-Control': 'no-store',
      },
      body: 'apiVersion: v1\nkind: Config\n',
    })
  })

  await page.goto('/')

  await page.getByRole('button', { name: 'Download Kubeconfig' }).click()
  const dialog = page.getByRole('dialog', { name: 'Download Kubeconfig' })
  await expect(dialog).toBeVisible()
  const clusterCheckbox = dialog.getByRole('checkbox').first()
  const clusterLabel = clusterCheckbox.locator('xpath=ancestor::label')
  await expect(clusterCheckbox).toBeVisible()
  await expect(clusterCheckbox).toBeChecked()
  await expect(clusterLabel).toHaveText(/\S/)

  await dialog.getByTestId('kubeconfig-ttl-86400').click()

  const download = page.waitForEvent('download')
  await dialog.getByRole('button', { name: 'Download', exact: true }).click()
  expect((await download).suggestedFilename()).toBe('kite-kubeconfig.yaml')
})

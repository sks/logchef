/**
 * Simple export function for compact log view
 * Exports raw log data without complex column processing
 */
export function exportCompactLogs(
  logs: Record<string, any>[],
  options?: {
    fileName?: string
    timestampField?: string
    severityField?: string
  }
) {
  const {
    fileName = 'logchef-logs-export',
    timestampField = 'timestamp',
    severityField = 'level'
  } = options || {}

  if (!logs || logs.length === 0) {
    return { success: false, error: 'No data to export' }
  }

  // Get all unique keys from the logs
  const allKeys = new Set<string>()
  logs.forEach(log => {
    Object.keys(log).forEach(key => allKeys.add(key))
  })

  // Order keys with timestamp first, then severity, then alphabetically
  const orderedKeys = Array.from(allKeys).sort((a, b) => {
    if (a === timestampField) return -1
    if (b === timestampField) return 1
    if (a === severityField) return -1
    if (b === severityField) return 1
    return a.localeCompare(b)
  })

  // Create CSV headers
  const headers = orderedKeys.map(key => `"${key.replace(/"/g, '""')}"`)

  // Create CSV rows
  const csvRows = [
    headers.join(','),
    ...logs.map(log => {
      return orderedKeys.map(key => {
        const value = log[key]
        if (value === null || value === undefined) {
          return ''
        } else if (typeof value === 'object') {
          return `"${JSON.stringify(value).replace(/"/g, '""')}"`
        }
        return `"${String(value).replace(/"/g, '""')}"`
      }).join(',')
    })
  ].join('\n')

  // Create and download the CSV file
  const csvBlob = new Blob([csvRows], { type: 'text/csv;charset=utf-8;' })
  const csvUrl = URL.createObjectURL(csvBlob)
  const link = document.createElement('a')
  link.href = csvUrl
  link.setAttribute('download', `${fileName}-${new Date().toISOString().slice(0, 10)}.csv`)
  document.body.appendChild(link)
  link.click()
  document.body.removeChild(link)
  URL.revokeObjectURL(csvUrl)

  return { success: true, rowCount: logs.length }
}

/**
 * Export function variants for compact mode
 */
export function exportCompactTableData(
  logs: Record<string, any>[],
  filteredLogs: Record<string, any>[],
  paginatedLogs: Record<string, any>[],
  options: {
    fileName?: string
    exportType: 'all' | 'filtered' | 'page'
    timestampField?: string
    severityField?: string
  }
) {
  let dataToExport: Record<string, any>[]
  let filePrefix = 'logchef-logs'

  switch (options.exportType) {
    case 'all':
      dataToExport = logs
      filePrefix = 'logchef-logs-all'
      break
    case 'filtered':
      dataToExport = filteredLogs
      filePrefix = 'logchef-logs-filtered'
      break
    case 'page':
      dataToExport = paginatedLogs
      filePrefix = 'logchef-logs-page'
      break
    default:
      dataToExport = logs
  }

  return exportCompactLogs(dataToExport, {
    fileName: options.fileName || filePrefix,
    timestampField: options.timestampField,
    severityField: options.severityField
  })
}
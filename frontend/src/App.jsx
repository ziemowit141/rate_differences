import { useEffect, useRef, useState } from 'react'
import './App.css'

function App() {
  const [data, setData] = useState(null)
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState('')
  const [uploading, setUploading] = useState(false)
  const [uploadError, setUploadError] = useState('')
  const [uploadResults, setUploadResults] = useState([])
  const [deleting, setDeleting] = useState('')
  const [tranches, setTranches] = useState([
    { key: 1, date: '', amount: '', rate: '', status: 'idle' },
  ])
  const [trancheError, setTrancheError] = useState('')
  const [trancheStatus, setTrancheStatus] = useState('')
  const [report, setReport] = useState(null)
  const nextTrancheKeyRef = useRef(2)

  const loadTransactions = async () => {
    setLoading(true)
    setError('')
    try {
      const response = await fetch('/transactions')
      if (!response.ok) {
        throw new Error(`HTTP ${response.status}`)
      }
      const payload = await response.json()
      setData(payload)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Unknown error')
    } finally {
      setLoading(false)
    }
  }

  const handleUpload = async (event) => {
    const files = Array.from(event.target.files || [])
    if (files.length === 0) {
      return
    }
    await uploadFiles(files)
    event.target.value = ''
  }

  const uploadFiles = async (files) => {
    if (!files.length) return
    setUploading(true)
    setUploadError('')
    setUploadResults([])
    try {
      const formData = new FormData()
      files.forEach((file) => formData.append('files', file))
      const response = await fetch('/upload', {
        method: 'POST',
        body: formData,
      })
      if (!response.ok) {
        throw new Error(`HTTP ${response.status}`)
      }
      const payload = await response.json()
      setUploadResults(payload.files || [])
      await loadTransactions()
    } catch (err) {
      setUploadError(err instanceof Error ? err.message : 'Unknown error')
    } finally {
      setUploading(false)
    }
  }

  const handleDrop = async (event) => {
    event.preventDefault()
    const files = Array.from(event.dataTransfer.files || []).filter(
      (file) => file.type === 'application/pdf'
    )
    if (!files.length || uploading) return
    await uploadFiles(files)
  }

  const handleDragOver = (event) => {
    event.preventDefault()
  }

  const handleDelete = async (baseName) => {
    if (!baseName || deleting) return
    setDeleting(baseName)
    setError('')
    try {
      const response = await fetch(`/files/${encodeURIComponent(baseName)}`, {
        method: 'DELETE',
      })
      if (!response.ok) {
        throw new Error(`HTTP ${response.status}`)
      }
      await loadTransactions()
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Unknown error')
    } finally {
      setDeleting('')
    }
  }

  const updateTranche = (index, field, value) => {
    setTranches((current) =>
      current.map((row, rowIndex) =>
        rowIndex === index ? { ...row, [field]: value } : row
      )
    )
  }

  const addTranche = () => {
    setTranches((current) => [
      ...current,
      { key: nextTrancheKeyRef.current, date: '', amount: '', rate: '', status: 'idle' },
    ])
    nextTrancheKeyRef.current += 1
  }

  const removeTranche = (index) => {
    setTranches((current) => current.filter((_, rowIndex) => rowIndex !== index))
  }

  const calculateReport = async () => {
    setTrancheError('')
    setTrancheStatus('')
    setReport(null)
    const payload = tranches
      .map((row) => ({
        date: row.date,
        amount: Number(row.amount),
        rate: Number(row.rate),
      }))
      .filter((row) => row.date && !Number.isNaN(row.amount) && !Number.isNaN(row.rate))
    if (!payload.length) {
      setTrancheError('Add at least one complete tranche row.')
      return
    }
    try {
      const response = await fetch('/calculate', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ tranches: payload }),
      })
      if (!response.ok) {
        throw new Error(`HTTP ${response.status}`)
      }
      const result = await response.json()
      setReport(result)
      setTrancheStatus('Report calculated.')
    } catch (err) {
      setTrancheError(err instanceof Error ? err.message : 'Unknown error')
    }
  }

  useEffect(() => {
    void loadTransactions()
  }, [])

  return (
    <div className="app">
      <header className="app-header">
        <div>
          <p className="eyebrow">MT940 Viewer</p>
          <h1>Transactions from your statements</h1>
          <p className="subhead">
            This view pulls data from the Go API at <code>/transactions</code>.
          </p>
        </div>
        <div className="header-actions">
          <label className="upload">
            <input
              type="file"
              accept="application/pdf"
              multiple
              onChange={handleUpload}
              disabled={uploading}
            />
            <span className="button button-secondary">
              {uploading ? 'Uploading…' : 'Upload PDFs'}
            </span>
          </label>
          <button className="button" onClick={loadTransactions} disabled={loading}>
            {loading ? 'Refreshing…' : 'Refresh'}
          </button>
        </div>
      </header>

      <section
        className={`dropzone ${uploading ? 'dropzone-disabled' : ''}`}
        onDrop={handleDrop}
        onDragOver={handleDragOver}
      >
        <div>
          <h2>Drag & drop PDFs</h2>
          <p>Drop multiple Citibank statements here or use the upload button.</p>
        </div>
      </section>

      <section className="tranche-card">
        <div className="tranche-header">
          <div>
            <p className="file-label">Tranches</p>
            <h2>Input tranches for FX calculations</h2>
          </div>
          <button className="button button-secondary" onClick={addTranche}>
            Add row
          </button>
        </div>
        <div className="tranche-table">
          <div className="tranche-row tranche-head">
            <span>Date</span>
            <span>Amount</span>
            <span>Rate</span>
            <span></span>
          </div>
          {tranches.map((row, index) => (
            <div
              className="tranche-row"
              key={`tranche-${row.key}`}
            >
              <input
                type="date"
                value={row.date}
                onChange={(event) => updateTranche(index, 'date', event.target.value)}
              />
              <input
                type="number"
                step="0.01"
                placeholder="Amount"
                value={row.amount}
                onChange={(event) => updateTranche(index, 'amount', event.target.value)}
              />
              <input
                type="number"
                step="0.0001"
                placeholder="Rate"
                value={row.rate}
                onChange={(event) => updateTranche(index, 'rate', event.target.value)}
              />
              <button
                className="button button-tertiary"
                onClick={() => removeTranche(index)}
                disabled={tranches.length === 1}
              >
                Remove
              </button>
            </div>
          ))}
        </div>
        <div className="tranche-actions">
          <button className="button" onClick={calculateReport}>
            Calculate
          </button>
          {trancheStatus && <span className="status-text">{trancheStatus}</span>}
          {trancheError && <span className="error-text">{trancheError}</span>}
        </div>
      </section>

      {report && (
        <section className="report-card">
          <div className="report-header">
            <div>
              <p className="file-label">Report</p>
              <h2>FX differences summary</h2>
            </div>
          </div>
          {report.error ? (
            <div className="panel panel-error">
              <strong>Calculation error:</strong> {report.error}
            </div>
          ) : null}
          <div className="report-grid">
            <div className="report-item">
              <span>Final PLN (gain/loss)</span>
              <strong>{report.summary?.total_fx_difference?.toFixed?.(2) ?? report.summary?.total_fx_difference}</strong>
            </div>
            <div className="report-item">
              <span>Total outflow</span>
              <strong>{report.summary?.total_outflow?.toFixed?.(2) ?? report.summary?.total_outflow}</strong>
            </div>
            <div className="report-item">
              <span>Total covered</span>
              <strong>{report.summary?.total_covered?.toFixed?.(2) ?? report.summary?.total_covered}</strong>
            </div>
            <div className="report-item">
              <span>Missing coverage</span>
              <strong>{report.summary?.missing_coverage?.toFixed?.(2) ?? report.summary?.missing_coverage}</strong>
            </div>
          </div>
          {report.warnings?.length ? (
            <div className="panel panel-error">
              <strong>Warnings:</strong>
              <ul>
                {report.warnings.map((warning, index) => (
                  <li key={`warn-${index}`}>{warning}</li>
                ))}
              </ul>
            </div>
          ) : null}
          <div className="report-subtitle">
            <h3>Tranche usage history</h3>
            <p>Outgoing transactions are grouped under the tranches they consumed.</p>
          </div>
          <div className="tranche-history">
            {report.tranches?.map((tranche, index) => (
              <div className="tranche-history-card" key={`tranche-${index}`}>
                <div className="tranche-history-header">
                  <div>
                    <strong>{tranche.date}</strong>
                    <span className="pill pill-muted">
                      {tranche.source === 'statement' ? 'Statement tranche' : 'Manual tranche'}
                    </span>
                  </div>
                  <span className="pill">
                    Remaining {tranche.remaining?.toFixed?.(2) ?? tranche.remaining}
                  </span>
                </div>
                <div className="tranche-history-meta">
                  <span>Amount: {tranche.amount?.toFixed?.(2) ?? tranche.amount}</span>
                  <span>Rate: {tranche.rate?.toFixed?.(4) ?? tranche.rate}</span>
                  {tranche.source_note ? <span>{tranche.source_note}</span> : null}
                </div>
                  {tranche.usages?.length ? (
                    <div className="tranche-usage-list">
                      {tranche.usages.map((usage, usageIndex) => (
                        <div className="tranche-usage" key={`usage-${usageIndex}`}>
                          <span>{usage.transaction_date}</span>
                          <span className="mono">{usage.transaction_ref || '—'}</span>
                          <span>{usage.amount_used?.toFixed?.(2) ?? usage.amount_used}</span>
                          <span>
                            FX {usage.fx_difference?.toFixed?.(2) ?? usage.fx_difference}
                          </span>
                          <span className="usage-formula">
                            ({usage.nbp_rate?.toFixed?.(4) ?? usage.nbp_rate} -{' '}
                            {tranche.rate?.toFixed?.(4) ?? tranche.rate}) ×{' '}
                            {usage.amount_used?.toFixed?.(2) ?? usage.amount_used}
                          </span>
                          <span>Remaining {usage.remaining?.toFixed?.(2) ?? usage.remaining}</span>
                        </div>
                      ))}
                    </div>
                ) : (
                  <div className="muted">No outflows assigned yet.</div>
                )}
              </div>
            ))}
          </div>
        </section>
      )}

      {uploadError && (
        <div className="panel panel-error">
          <strong>Upload failed:</strong> {uploadError}
        </div>
      )}

      {uploadResults.length > 0 && (
        <div className="panel panel-upload">
          <h3>Upload results</h3>
          <ul>
            {uploadResults.map((result) => (
              <li key={result.source}>
                <strong>{result.source}</strong>{' '}
                {result.error ? (
                  <span className="error-text">{result.error}</span>
                ) : (
                  <span>
                    → {result.pdf_path} → {result.mt940_path}
                  </span>
                )}
              </li>
            ))}
          </ul>
        </div>
      )}

      {error && (
        <div className="panel panel-error">
          <strong>Load failed:</strong> {error}
        </div>
      )}

      {!data && !loading && !error && (
        <div className="panel">No data yet. Click refresh to try again.</div>
      )}

      {data?.files?.length ? (
        <div className="files">
          {data.files.map((file) => (
            <section key={file.file} className="file-card">
              <div className="file-header">
                <div>
                  <p className="file-label">Statement file</p>
                  <h2>{file.base_name || file.file}</h2>
                  <p className="file-rate">
                    NBP USD:{" "}
                    {file.nbp_rate
                      ? `${file.nbp_rate} (as of ${file.nbp_date || '—'})`
                      : file.nbp_error
                      ? `Unavailable (${file.nbp_error})`
                      : '—'}
                  </p>
                </div>
                <div className="file-actions">
                  <span className="pill">
                    {file.transactions?.length ?? 0} transactions
                  </span>
                  <button
                    className="button button-tertiary"
                    onClick={() => handleDelete(file.base_name)}
                    disabled={deleting === file.base_name}
                  >
                    {deleting === file.base_name ? 'Deleting…' : 'Delete'}
                  </button>
                </div>
              </div>
              {file.error ? (
                <div className="panel panel-error">{file.error}</div>
              ) : (
                <div className="table">
                  <div className="table-row table-head">
                    <span>Value date</span>
                    <span>Entry date</span>
                    <span>D/C</span>
                    <span>Amount</span>
                    <span>Code</span>
                    <span>Reference</span>
                    <span>Details</span>
                  </div>
                  {file.transactions?.map((txn, index) => (
                    <div className="table-row" key={`${file.file}-${index}`}>
                      <span>{txn.value_date || '—'}</span>
                      <span>{txn.entry_date || '—'}</span>
                      <span>{txn.dc_mark || '—'}</span>
                      <span>{txn.amount || '—'}</span>
                      <span>{txn.code || '—'}</span>
                      <span className="mono">{txn.reference || '—'}</span>
                      <span className="details">{txn.details || '—'}</span>
                    </div>
                  ))}
                </div>
              )}
            </section>
          ))}
        </div>
      ) : null}
    </div>
  )
}

export default App

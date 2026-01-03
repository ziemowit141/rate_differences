import { useEffect, useRef, useState } from 'react'
import './App.css'

function App() {
  const [lang, setLang] = useState('pl')
  const [data, setData] = useState(null)
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState('')
  const [uploading, setUploading] = useState(false)
  const [uploadError, setUploadError] = useState('')
  const [deleting, setDeleting] = useState('')
  const [tranches, setTranches] = useState([
    { key: 1, date: '', amount: '', rate: '', status: 'idle' },
  ])
  const [trancheError, setTrancheError] = useState('')
  const [trancheStatus, setTrancheStatus] = useState('')
  const [report, setReport] = useState(null)
  const nextTrancheKeyRef = useRef(2)

  const copy = {
    pl: {
      title: 'Kalkulator różnic kursowych',
      subtitle:
        'Wgraj wyciągi i uzupełnij transze. Program obliczy wynik końcowy i pokaże historię rozliczeń.',
      upload: 'Wgraj PDF',
      refresh: 'Odśwież',
      dropTitle: 'Przeciągnij i upuść PDF',
      dropText: 'Możesz dodać wiele wyciągów naraz.',
      tranchesTitle: 'Transze',
      tranchesSubtitle: 'Wprowadź transze zakupu waluty (data, kwota, kurs).',
      addRow: 'Dodaj wiersz',
      date: 'Data',
      amount: 'Kwota',
      rate: 'Kurs',
      remove: 'Usuń',
      calculate: 'Oblicz',
      reportTitle: 'Raport',
      summaryTitle: 'Podsumowanie różnic kursowych',
      finalPln: 'Wynik końcowy (PLN)',
      totalOutflow: 'Wydatki łącznie',
      totalCovered: 'Pokryte wydatki',
      missingCoverage: 'Brakujące pokrycie',
      warnings: 'Ostrzeżenia',
      trancheHistoryTitle: 'Historia rozliczeń transz',
      trancheHistorySubtitle: 'Wydatki przypisane do transz (FIFO).',
      manualTranche: 'Transza ręczna',
      statementTranche: 'Transza z wyciągu',
      remaining: 'Pozostało',
      reportCalculated: 'Raport policzony.',
      calcError: 'Błąd obliczeń',
      statementFile: 'Wyciąg',
      transactions: 'transakcji',
      delete: 'Usuń',
      dc: 'D/C',
      code: 'Kod',
      reference: 'Numer referencyjny',
      valueDate: 'Data waluty',
      entryDate: 'Data księg.',
      details: 'Opis',
      nbpUsd: 'NBP USD',
      asOf: 'z dnia',
      formulaLabel: 'Wzór',
      fx: 'Różnica',
    },
    en: {
      title: 'FX Differences Calculator',
      subtitle:
        'Upload statements and add tranches. The report shows the final result and allocation history.',
      upload: 'Upload PDFs',
      refresh: 'Refresh',
      dropTitle: 'Drag & drop PDFs',
      dropText: 'You can upload multiple statements at once.',
      tranchesTitle: 'Tranches',
      tranchesSubtitle: 'Enter currency purchase tranches (date, amount, rate).',
      addRow: 'Add row',
      date: 'Date',
      amount: 'Amount',
      rate: 'Rate',
      remove: 'Remove',
      calculate: 'Calculate',
      reportTitle: 'Report',
      summaryTitle: 'FX differences summary',
      finalPln: 'Final PLN (gain/loss)',
      totalOutflow: 'Total outflow',
      totalCovered: 'Covered outflow',
      missingCoverage: 'Missing coverage',
      warnings: 'Warnings',
      trancheHistoryTitle: 'Tranche usage history',
      trancheHistorySubtitle: 'Outflows assigned to tranches (FIFO).',
      manualTranche: 'Manual tranche',
      statementTranche: 'Statement tranche',
      remaining: 'Remaining',
      reportCalculated: 'Report calculated.',
      calcError: 'Calculation error',
      statementFile: 'Statement',
      transactions: 'transactions',
      delete: 'Delete',
      valueDate: 'Value date',
      entryDate: 'Posting date',
      dc: 'D/C',
      code: 'Code',
      reference: 'Reference',
      details: 'Details',
      nbpUsd: 'NBP USD',
      asOf: 'as of',
      formulaLabel: 'Formula',
      fx: 'FX diff',
    },
  }

  const t = copy[lang]

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
          <p className="eyebrow">FX Toolkit</p>
          <h1>{t.title}</h1>
          <p className="subhead">{t.subtitle}</p>
        </div>
        <div className="header-actions">
          <div className="lang-toggle">
            <button
              className={`button button-tertiary ${lang === 'pl' ? 'active' : ''}`}
              onClick={() => setLang('pl')}
            >
              PL
            </button>
            <button
              className={`button button-tertiary ${lang === 'en' ? 'active' : ''}`}
              onClick={() => setLang('en')}
            >
              EN
            </button>
          </div>
          <label className="upload">
            <input
              type="file"
              accept="application/pdf"
              multiple
              onChange={handleUpload}
              disabled={uploading}
            />
            <span className="button button-secondary">
              {uploading ? 'Uploading…' : t.upload}
            </span>
          </label>
          <button className="button" onClick={loadTransactions} disabled={loading}>
            {loading ? 'Refreshing…' : t.refresh}
          </button>
        </div>
      </header>

      <section
        className={`dropzone ${uploading ? 'dropzone-disabled' : ''}`}
        onDrop={handleDrop}
        onDragOver={handleDragOver}
      >
        <div>
          <h2>{t.dropTitle}</h2>
          <p>{t.dropText}</p>
        </div>
      </section>

      <section className="tranche-card">
        <div className="tranche-header">
          <div>
            <p className="file-label">{t.tranchesTitle}</p>
            <h2>{t.tranchesSubtitle}</h2>
          </div>
          <button className="button button-secondary" onClick={addTranche}>
            {t.addRow}
          </button>
        </div>
        <div className="tranche-table">
          <div className="tranche-row tranche-head">
            <span>{t.date}</span>
            <span>{t.amount}</span>
            <span>{t.rate}</span>
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
                placeholder={t.amount}
                value={row.amount}
                onChange={(event) => updateTranche(index, 'amount', event.target.value)}
              />
              <input
                type="number"
                step="0.0001"
                placeholder={t.rate}
                value={row.rate}
                onChange={(event) => updateTranche(index, 'rate', event.target.value)}
              />
              <button className="button button-tertiary" onClick={() => removeTranche(index)}>
                {t.remove}
              </button>
            </div>
          ))}
        </div>
        <div className="tranche-actions">
          <button className="button" onClick={calculateReport}>
            {t.calculate}
          </button>
          {trancheStatus && <span className="status-text">{t.reportCalculated}</span>}
          {trancheError && <span className="error-text">{trancheError}</span>}
        </div>
      </section>

      {report && (
        <section className="report-card">
          <div className="report-header">
            <div>
              <p className="file-label">{t.reportTitle}</p>
              <h2>{t.summaryTitle}</h2>
            </div>
          </div>
          {report.error ? (
            <div className="panel panel-error">
              <strong>{t.calcError}:</strong> {report.error}
            </div>
          ) : null}
          <div className="report-grid">
            <div
              className={`report-item ${
                report.summary?.total_fx_difference > 0
                  ? 'gain'
                  : report.summary?.total_fx_difference < 0
                  ? 'loss'
                  : ''
              }`}
            >
              <span>{t.finalPln}</span>
              <strong
                className={
                  report.summary?.total_fx_difference > 0
                    ? 'gain'
                    : report.summary?.total_fx_difference < 0
                    ? 'loss'
                    : ''
                }
              >
                {report.summary?.total_fx_difference?.toFixed?.(2) ?? report.summary?.total_fx_difference}
              </strong>
            </div>
            <div className="report-item">
              <span>{t.totalOutflow}</span>
              <strong>{report.summary?.total_outflow?.toFixed?.(2) ?? report.summary?.total_outflow}</strong>
            </div>
            <div className="report-item">
              <span>{t.totalCovered}</span>
              <strong>{report.summary?.total_covered?.toFixed?.(2) ?? report.summary?.total_covered}</strong>
            </div>
            <div className="report-item">
              <span>{t.missingCoverage}</span>
              <strong>{report.summary?.missing_coverage?.toFixed?.(2) ?? report.summary?.missing_coverage}</strong>
            </div>
          </div>
          {report.warnings?.length ? (
            <div className="panel panel-error">
              <strong>{t.warnings}:</strong>
              <ul>
                {report.warnings.map((warning, index) => (
                  <li key={`warn-${index}`}>{warning}</li>
                ))}
              </ul>
            </div>
          ) : null}
          <div className="report-subtitle">
            <h3>{t.trancheHistoryTitle}</h3>
            <p>{t.trancheHistorySubtitle}</p>
          </div>
          <div className="tranche-history">
            {report.tranches?.map((tranche, index) => (
              <div className="tranche-history-card" key={`tranche-${index}`}>
                <div className="tranche-history-header">
                  <div>
                    <strong>{tranche.date}</strong>
                    <span className="pill pill-muted">
                      {tranche.source === 'statement' ? t.statementTranche : t.manualTranche}
                    </span>
                  </div>
                  <span className="pill">
                    {t.remaining} {tranche.remaining?.toFixed?.(2) ?? tranche.remaining}
                  </span>
                </div>
                <div className="tranche-history-meta">
                  <span>{t.amount}: {tranche.amount?.toFixed?.(2) ?? tranche.amount}</span>
                  <span>{t.rate}: {tranche.rate?.toFixed?.(4) ?? tranche.rate}</span>
                  {tranche.source_note ? (
                    <span>
                      {lang === 'pl' ? 'Automatycznie z wyciągu' : 'Auto from statement'}
                    </span>
                  ) : null}
                </div>
                {tranche.usages?.length ? (
                  <div className="tranche-usage-list">
                    {tranche.usages.map((usage, usageIndex) => (
                      <div className="tranche-usage" key={`usage-${usageIndex}`}>
                        <span>{usage.transaction_date}</span>
                        <span className="mono">{usage.transaction_ref || '—'}</span>
                        <span>{usage.amount_used?.toFixed?.(2) ?? usage.amount_used}</span>
                        <span>
                          {t.fx} {usage.fx_difference?.toFixed?.(2) ?? usage.fx_difference}
                        </span>
                        <span className="usage-formula">
                          {t.formulaLabel}:{' '}
                          ({usage.nbp_rate?.toFixed?.(4) ?? usage.nbp_rate} -{' '}
                          {tranche.rate?.toFixed?.(4) ?? tranche.rate}) ×{' '}
                          {usage.amount_used?.toFixed?.(2) ?? usage.amount_used}
                        </span>
                        <span>{t.remaining} {usage.remaining?.toFixed?.(2) ?? usage.remaining}</span>
                      </div>
                    ))}
                  </div>
                ) : (
                  <div className="muted">
                    {lang === 'pl' ? 'Brak przypisanych wydatków.' : 'No outflows assigned yet.'}
                  </div>
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
                  <p className="file-label">{t.statementFile}</p>
                  <h2>{file.base_name || file.file}</h2>
                  <p className="file-rate">
                    {t.nbpUsd}:{' '}
                    {file.nbp_rate
                      ? `${file.nbp_rate} (${t.asOf} ${file.nbp_date || '—'})`
                      : file.nbp_error
                      ? `Unavailable (${file.nbp_error})`
                      : '—'}
                  </p>
                </div>
                <div className="file-actions">
                  <span className="pill">
                    {file.transactions?.length ?? 0} {t.transactions}
                  </span>
                  <button
                    className="button button-tertiary"
                    onClick={() => handleDelete(file.base_name)}
                    disabled={deleting === file.base_name}
                  >
                    {deleting === file.base_name ? 'Deleting…' : t.delete}
                  </button>
                </div>
              </div>
              {file.error ? (
                <div className="panel panel-error">{file.error}</div>
              ) : (
                <div className="table">
                  <div className="table-row table-head">
                    <span>{t.valueDate}</span>
                    <span>{t.entryDate}</span>
                    <span>{t.dc}</span>
                    <span>{t.amount}</span>
                    <span>{t.code}</span>
                    <span>{t.reference}</span>
                    <span>{t.details}</span>
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

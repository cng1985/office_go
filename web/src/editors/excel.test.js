import assert from 'node:assert/strict'
import test from 'node:test'
import {
  deleteRows,
  fillRange,
  findCell,
  growSheet,
  insertRows,
  normalizeWorkbook,
  pasteCells,
  pasteCut,
  pasteValues,
  renameSheetRefs,
  shiftFormula,
  sortRows,
} from './excel.js'

test('legacy sheet becomes a workbook', () => {
  const workbook = normalizeWorkbook({
    sheet_name: '预算',
    row_count: 30,
    col_count: 12,
    cells: [{ row: 1, col: 1, input: '100', bold: false }],
  })
  assert.deepEqual(workbook, {
    sheets: [{
      name: '预算',
      row_count: 30,
      col_count: 12,
      cells: [{ row: 1, col: 1, input: '100' }],
    }],
  })
})

test('insert and delete rows adjust formulas', () => {
  const sheet = {
    row_count: 5,
    col_count: 3,
    cells: [
      { row: 1, col: 1, input: '1' },
      { row: 3, col: 1, input: '=A1+$A$3' },
    ],
  }
  assert.equal(insertRows(sheet, 2, 1), true)
  assert.equal(sheet.cells.find((cell) => cell.row === 4).input, '=A1+$A$4')
  assert.equal(deleteRows(sheet, 1, 1), true)
  assert.equal(sheet.cells.find((cell) => cell.input.startsWith('=')).input, '=#REF!+$A$3')
  const cross = { name: 'Sheet1', row_count: 4, col_count: 2, cells: [{ row: 1, col: 1, input: '=Sheet2!A1+A2' }] }
  insertRows(cross, 1, 1)
  assert.equal(cross.cells[0].input, '=Sheet2!A1+A3')
})

test('structural edits update references that target the edited sheet', () => {
  const sheet1 = { name: 'Sheet1', row_count: 4, col_count: 2, cells: [{ row: 1, col: 1, input: '10' }] }
  const sheet2 = {
    name: 'Sheet2',
    row_count: 4,
    col_count: 2,
    cells: [
      { row: 1, col: 1, input: '=Sheet1!A1' },
      { row: 2, col: 1, input: '=A1' },
    ],
  }
  const workbook = { sheets: [sheet1, sheet2] }
  insertRows(sheet1, 1, 1, workbook)
  assert.equal(sheet1.cells[0].row, 2)
  assert.equal(sheet2.cells[0].input, '=Sheet1!A2')
  assert.equal(sheet2.cells[1].input, '=A1')
})

test('sorting shifts relative references and keeps absolute ones', () => {
  const sheet = {
    name: 'Sheet1',
    row_count: 4,
    col_count: 2,
    cells: [
      { row: 1, col: 1, input: '2' },
      { row: 2, col: 1, input: '1' },
      { row: 1, col: 2, input: '=A1*10' },
      { row: 2, col: 2, input: '=A2*10+$A$1' },
    ],
  }
  sortRows(sheet, { r1: 1, c1: 1, r2: 2, c2: 2 }, 1, 'asc', { sheets: [sheet] })
  assert.equal(findCell(sheet, 1, 1).input, '1')
  assert.equal(findCell(sheet, 2, 1).input, '2')
  assert.equal(findCell(sheet, 1, 2).input, '=A1*10+$A$1')
  assert.equal(findCell(sheet, 2, 2).input, '=A2*10')
})

test('cutting onto an overlapping range keeps the pasted cells', () => {
  const sheet = {
    row_count: 5,
    col_count: 2,
    cells: [
      { row: 1, col: 1, input: 'one' },
      { row: 2, col: 1, input: 'two' },
    ],
  }
  const matrix = [
    [{ row: 1, col: 1, input: 'one' }],
    [{ row: 2, col: 1, input: 'two' }],
  ]
  pasteCut(sheet, sheet, 2, 1, matrix)
  assert.equal(findCell(sheet, 1, 1), null)
  assert.equal(findCell(sheet, 2, 1).input, 'one')
  assert.equal(findCell(sheet, 3, 1).input, 'two')
})

test('renaming a sheet rewrites qualified references', () => {
  const workbook = {
    sheets: [
      { name: 'Sheet2', row_count: 2, col_count: 2, cells: [] },
      { name: 'Sheet1', row_count: 2, col_count: 2, cells: [{ row: 1, col: 1, input: '=Sheet2!A1' }, { row: 2, col: 1, input: "='Old Name'!B2" }] },
    ],
  }
  renameSheetRefs(workbook, 'Sheet2', 'Q1')
  assert.equal(workbook.sheets[1].cells[0].input, '=Q1!A1')
  renameSheetRefs(workbook, 'Old Name', 'New Name')
  assert.equal(workbook.sheets[1].cells[1].input, "='New Name'!B2")
})

test('growing or pasting columns keeps the width list aligned', () => {
  const widths = Array.from({ length: 16 }, (_, index) => (index === 0 ? 80 : 0))
  const sheet = { row_count: 4, col_count: 16, col_widths: widths, cells: [] }
  growSheet(sheet, 0, 2)
  assert.equal(sheet.col_count, 18)
  assert.equal(sheet.col_widths.length, 18)
  assert.equal(sheet.col_widths[0], 80)
  assert.equal(sheet.col_widths[17], 0)
  pasteValues(sheet, 1, 20, [['tail']])
  assert.equal(sheet.col_count, 20)
  assert.equal(sheet.col_widths.length, 20)
  assert.equal(findCell(sheet, 1, 20).input, 'tail')
})

test('fill continues a numeric series and shifts relative formulas', () => {
  const sheet = {
    row_count: 8,
    col_count: 3,
    cells: [
      { row: 1, col: 1, input: '1' },
      { row: 2, col: 1, input: '3' },
      { row: 1, col: 2, input: '=A1' },
      { row: 2, col: 2, input: '=$A$1' },
    ],
  }
  fillRange(sheet, { r1: 1, c1: 1, r2: 2, c2: 2 }, { r1: 1, c1: 1, r2: 4, c2: 2 })
  assert.equal(sheet.cells.find((cell) => cell.row === 3 && cell.col === 1).input, '5')
  assert.equal(sheet.cells.find((cell) => cell.row === 4 && cell.col === 1).input, '7')
  assert.equal(sheet.cells.find((cell) => cell.row === 3 && cell.col === 2).input, '=A3')
  assert.equal(sheet.cells.find((cell) => cell.row === 4 && cell.col === 2).input, '=$A$1')
  assert.equal(shiftFormula('=A1', 1, 0), '=A2')
})

test('paste shifts formulas from their original cells', () => {
  const sheet = { row_count: 6, col_count: 4, cells: [] }
  pasteCells(sheet, 3, 2, [[{ row: 1, col: 1, input: '=B2+$C$3' }]])
  assert.equal(sheet.cells[0].input, '=C4+$C$3')
  assert.equal(sheet.cells[0].row, 3)
  assert.equal(sheet.cells[0].col, 2)
})

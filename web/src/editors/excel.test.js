import assert from 'node:assert/strict'
import test from 'node:test'
import {
  deleteRows,
  fillRange,
  insertRows,
  normalizeWorkbook,
  pasteCells,
  shiftFormula,
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

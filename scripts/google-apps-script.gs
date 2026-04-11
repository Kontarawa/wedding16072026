/**
 * Разверните как веб-приложение: Развернуть → Новое развертывание → Тип: Веб-приложение.
 * Выполнять от имени: «Я», у кого есть доступ: «Все» (в т.ч. анонимные).
 * Скопируйте URL вида …/exec в GOOGLE_SHEETS_WEBAPP_URL или в <meta name="google-sheets-webapp" content="…">.
 *
 * Таблица: https://docs.google.com/spreadsheets/d/1tgBm-iP2UBEDJakOD5dgBlKRHq6RkawUYdxogI01zZI/edit
 */

/** Открытие ссылки в браузере (GET) — не ошибка: анкеты приходят только методом POST с сайта. */
function doGet() {
  return ContentService
    .createTextOutput('OK. Этот адрес принимает ответы анкеты методом POST с приглашения. Откройте сами приглашение и отправьте форму.')
    .setMimeType(ContentService.MimeType.TEXT);
}

function doPost(e) {
  try {
    var raw = (e.postData && e.postData.contents) ? e.postData.contents : '{}';
    var data = JSON.parse(raw);
    var sheet = SpreadsheetApp.openById('1tgBm-iP2UBEDJakOD5dgBlKRHq6RkawUYdxogI01zZI').getSheets()[0];
    if (sheet.getLastRow() === 0) {
      sheet.appendRow(['submitted_at', 'hash', 'first_name', 'last_name', 'attendance', 'transfer', 'alcohol']);
    }
    sheet.appendRow([
      data.submitted_at || new Date().toISOString(),
      data.hash || '',
      data.first_name || '',
      data.last_name || '',
      data.attendance || '',
      data.transfer || '',
      data.alcohol || ''
    ]);
    return ContentService
      .createTextOutput(JSON.stringify({ ok: true }))
      .setMimeType(ContentService.MimeType.JSON);
  } catch (err) {
    return ContentService
      .createTextOutput(JSON.stringify({ ok: false, error: String(err) }))
      .setMimeType(ContentService.MimeType.JSON);
  }
}

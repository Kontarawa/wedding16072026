const SHEET_NAME = "Responses";
const TOKEN_PROP = "WEDDING_WEBHOOK_TOKEN";

function doGet() {
  return json_(200, { ok: true, message: "Webhook is running. Use POST with JSON." });
}

function doPost(e) {
  try {
    const body = JSON.parse((e && e.postData && e.postData.contents) || "{}");
    const want = String(PropertiesService.getScriptProperties().getProperty(TOKEN_PROP) || "").trim();
    const got = String(body.token || "").trim();
    if (!want || got !== want) {
      return json_(401, { ok: false, error: "unauthorized" });
    }

    const ss = SpreadsheetApp.getActiveSpreadsheet();
    const sheet = ss.getSheetByName(SHEET_NAME) || ss.insertSheet(SHEET_NAME);

    // Header (create once)
    if (sheet.getLastRow() === 0) {
      sheet.appendRow(["submitted_at", "first_name", "last_name", "attendance", "transfer", "alcohol", "hash"]);
    }

    sheet.appendRow([
      String(body.submitted_at || ""),
      String(body.first_name || ""),
      String(body.last_name || ""),
      String(body.attendance || ""),
      String(body.transfer || ""),
      String(body.alcohol || ""),
      String(body.hash || ""),
    ]);

    return json_(200, { ok: true });
  } catch (err) {
    return json_(500, { ok: false, error: String(err && err.message ? err.message : err) });
  }
}

function json_(code, obj) {
  return ContentService.createTextOutput(JSON.stringify(obj)).setMimeType(ContentService.MimeType.JSON);
}


param([int]$TaskProcessId)
# Startup-only evidence on a fresh cloud desktop, before test credentials are pasted.
Add-Type -AssemblyName UIAutomationClient
$condition = [System.Windows.Automation.PropertyCondition]::new([System.Windows.Automation.AutomationElement]::ProcessIdProperty, $TaskProcessId)
$items = [System.Windows.Automation.AutomationElement]::RootElement.FindAll([System.Windows.Automation.TreeScope]::Descendants, $condition)
$names = @()
foreach ($item in $items) {
    try { if ($item.Current.Name) { $names += $item.Current.Name } } catch {}
}
$names | Select-Object -Unique | ConvertTo-Json -Compress
Add-Type -AssemblyName System.Windows.Forms
Add-Type -AssemblyName System.Drawing
$rect = [System.Windows.Forms.SystemInformation]::VirtualScreen
$bitmap = [System.Drawing.Bitmap]::new($rect.Width, $rect.Height)
$graphics = [System.Drawing.Graphics]::FromImage($bitmap)
$graphics.CopyFromScreen($rect.X, $rect.Y, 0, 0, $bitmap.Size)
$bitmap.Save((Join-Path (Get-Location) 'native-evidence/desktop-start.png'))
$graphics.Dispose()
$bitmap.Dispose()

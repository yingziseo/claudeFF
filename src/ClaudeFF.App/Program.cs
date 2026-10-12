using System.Runtime.CompilerServices;
using System.Runtime.InteropServices;
using System.Windows;
using System.Windows.Threading;
namespace ClaudeFF.App;
public static class Program
{
    [STAThread] public static int Main(string[] args)
    {
        StartupDiagnostics.NonInteractive=args.Contains("--ui-smoke-test");
        StartupDiagnostics.Write("Managed entry reached; version="+typeof(Program).Assembly.GetName().Version+"; OS="+Environment.OSVersion+"; 64-bit="+Environment.Is64BitProcess);
        AppDomain.CurrentDomain.UnhandledException+=(s,e)=>StartupDiagnostics.Write(e.ExceptionObject.ToString()??"Unhandled exception");
        try{return Start(args);}
        catch(Exception ex){StartupDiagnostics.Fail(ex);return 1;}
    }
    // Keep WPF loading and window construction inside the outer exception boundary.
    [MethodImpl(MethodImplOptions.NoInlining)] private static int Start(string[] args)
    {
        if(args.Contains("--software-rendering"))System.Windows.Media.RenderOptions.ProcessRenderMode=System.Windows.Interop.RenderMode.SoftwareOnly;
        var app=new Application();
        app.DispatcherUnhandledException+=(s,e)=>{StartupDiagnostics.Fail(e.Exception);e.Handled=true;app.Shutdown(1);};
        StartupDiagnostics.Write("Creating main window");
        var window=new MainWindow();
        bool smokeQueued=false;
        window.ContentRendered+=(s,e)=>{
            StartupDiagnostics.Write("Main window rendered");
            if(args.Contains("--ui-smoke-test")&&!smokeQueued)
            {
                smokeQueued=true;
                window.Dispatcher.BeginInvoke(DispatcherPriority.ApplicationIdle,new Action(async()=>{
                    try
                    {
                        await window.SmokeTestPagesAsync(StartupDiagnostics.OutputDirectory);
                        StartupDiagnostics.Write("UI smoke test passed");app.Shutdown(0);
                    }
                    catch(Exception ex){StartupDiagnostics.Fail(ex);app.Shutdown(1);}
                }));
            }
        };
        return app.Run(window);
    }
}
internal static class StartupDiagnostics
{
    public static bool NonInteractive {get;set;}
    public static string OutputDirectory {get;}=Environment.GetEnvironmentVariable("PRIVACY_UI_TEST_OUTPUT") ?? Path.Combine(Environment.GetFolderPath(Environment.SpecialFolder.LocalApplicationData),"ClaudeFF","Logs");
    public static string LogPath => Path.Combine(OutputDirectory,"startup.log");
    public static void Write(string message)
    {
        try{Directory.CreateDirectory(Path.GetDirectoryName(LogPath)!);if(File.Exists(LogPath)&&new FileInfo(LogPath).Length>1_048_576)File.Move(LogPath,LogPath+".previous",true);File.AppendAllText(LogPath,DateTimeOffset.Now.ToString("O")+" "+message+Environment.NewLine);}catch{}
    }
    public static void Fail(Exception ex)
    {
        Write(ex.ToString());
        // Native dialog also works if WPF initialization itself failed.
        if(!NonInteractive)NativeMessageBox(IntPtr.Zero,"claudeFF 启动或界面出现异常。\n\n"+ex.GetType().Name+"："+ex.Message+"\n\n诊断日志："+LogPath+"\n可运行压缩包内的 Diagnose.cmd 获取启动诊断。","claudeFF · 启动诊断",0x10);
    }
    [DllImport("user32.dll",CharSet=CharSet.Unicode,EntryPoint="MessageBoxW")]private static extern int NativeMessageBox(IntPtr owner,string text,string caption,uint type);
}

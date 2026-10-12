using System.Diagnostics;
using System.Windows;
using System.Windows.Controls;
using System.Windows.Media;
using System.Windows.Automation;
using Microsoft.Win32;
using ClaudeFF.Core;
using ClaudeFF.Windows;

namespace ClaudeFF.App;
public sealed class MainWindow:Window
{
    private Settings settings;
    private StackPanel page=null!,actions=null!;
    private TextBlock status=null!,breadcrumb=null!;
    private Grid layout=null!;
    private TextBox sandboxHome=null!,claudePath=null!,proxyHost=null!,proxyPort=null!,proxyUser=null!;
    private PasswordBox proxyPassword=null!;
    private CheckBox browsers=null!,managedTcp=null!;
    private ListBox folders=null!;
    private int selectedPage;
    private bool busy;
    private string currentIp="尚未检测";
    private readonly List<Button> navButtons=[];
    private Report? lastReport;
    private readonly string[] labels=["隐私防护","网络连接","数据偏好","防护验证"];
    public MainWindow()
    {
        try{settings=LocalStore.Load();}catch{settings=new();}
        Title="claudeFF · 桌面隐私防护";Width=1120;Height=800;MinWidth=900;MinHeight=660;WindowStartupLocation=WindowStartupLocation.CenterScreen;
        Build();
        Closing+=(s,e)=>{if(busy){e.Cancel=true;status.Text="操作进行中，请等待完成后关闭窗口。";}};
    }
    internal async Task SmokeTestPagesAsync(string output)
    {
        Directory.CreateDirectory(output);Width=1280;Height=800;
        var results=new List<object>();
        for(int theme=0;theme<2;theme++)
        {
            settings.LightTheme=theme==1;Build();
            for(int i=0;i<labels.Length;i++)
            {
                selectedPage=i;RenderPage();
                if(i==0)
                {
                    await Dispatcher.InvokeAsync(UpdateLayout,System.Windows.Threading.DispatcherPriority.Render);await Task.Delay(180);
                    SaveScreenshot(output,(settings.LightTheme?"light":"dark")+"-home-default.png");
                }
                ExpandAll(page);
                await Dispatcher.InvokeAsync(UpdateLayout,System.Windows.Threading.DispatcherPriority.Render);
                await Task.Delay(180);
                var handle=new System.Windows.Interop.WindowInteropHelper(this).Handle;
                if(!IsVisible||handle==IntPtr.Zero||ActualWidth<900||ActualHeight<660)throw new InvalidOperationException("未创建可见的原生主窗口。");
                var name=(settings.LightTheme?"light":"dark")+"-page-"+i+".png";
                SaveScreenshot(output,name);
                results.Add(new{Theme=settings.LightTheme?"light":"dark",Page=labels[i],WindowHandle=handle.ToInt64(),Visible=IsVisible,Width=ActualWidth,Height=ActualHeight,Screenshot=name});
            }
        }
        Json.Write(Path.Combine(output,"ui-result.json"),new{Success=true,Pid=Environment.ProcessId,OS=Environment.OSVersion.ToString(),Architecture=System.Runtime.InteropServices.RuntimeInformation.ProcessArchitecture.ToString(),Checks=results});
    }
    private void SaveScreenshot(string output,string name)
    {
        var target=new System.Windows.Media.Imaging.RenderTargetBitmap((int)Math.Ceiling(layout.ActualWidth),(int)Math.Ceiling(layout.ActualHeight),96,96,PixelFormats.Pbgra32);
        target.Render(layout);var encoder=new System.Windows.Media.Imaging.PngBitmapEncoder();encoder.Frames.Add(System.Windows.Media.Imaging.BitmapFrame.Create(target));
        using var stream=File.Create(Path.Combine(output,name));encoder.Save(stream);
    }
    private static void ExpandAll(System.Windows.DependencyObject root)
    {
        if(root is Expander expander)expander.IsExpanded=true;
        for(int i=0;i<VisualTreeHelper.GetChildrenCount(root);i++)ExpandAll(VisualTreeHelper.GetChild(root,i));
    }
    private Brush Brush(string name)=>(Brush)Application.Current.Resources[name];
    private FrameworkElement Glyph(string name,double size=20)
    {
        var canvas=new Canvas{Width=24,Height=24};
        canvas.Children.Add(new System.Windows.Shapes.Path{Data=Geometry.Parse(Icons.Paths[name]),Stroke=Brush("Muted"),StrokeThickness=1.7,StrokeStartLineCap=PenLineCap.Round,StrokeEndLineCap=PenLineCap.Round,StrokeLineJoin=PenLineJoin.Round});
        return new Viewbox{Child=canvas,Width=size,Height=size,Margin=new Thickness(0,0,12,0)};
    }
    private FrameworkElement Logo(double size=28)=>new System.Windows.Shapes.Path{Data=Geometry.Parse(Brand.Geometry),Fill=Brush("Accent"),Stretch=Stretch.Uniform,Width=size,Height=size,Margin=new Thickness(0,0,12,0)};
    private void Build()
    {
        Theme.Apply(settings.LightTheme);Background=Brush("Bg");Foreground=Brush("Text");
        layout=new Grid{Background=Brush("Bg")};Content=layout;
        layout.ColumnDefinitions.Add(new(){Width=new GridLength(204)});layout.ColumnDefinitions.Add(new(){Width=new GridLength(1,GridUnitType.Star)});
        var side=new DockPanel{Margin=new Thickness(14,28,14,18)};
        var sideBorder=new Border{Background=Brush("Sidebar"),BorderBrush=Brush("Line"),BorderThickness=new Thickness(0,0,1,0),Child=side};layout.Children.Add(sideBorder);
        var brand=new StackPanel{Orientation=Orientation.Horizontal,Margin=new Thickness(12,0,0,40)};brand.Children.Add(Logo());brand.Children.Add(Text("claudeFF",23,true));DockPanel.SetDock(brand,Dock.Top);side.Children.Add(brand);
        var bottom=new StackPanel();DockPanel.SetDock(bottom,Dock.Bottom);side.Children.Add(bottom);
        bottom.Children.Add(Button(settings.LightTheme?"切换深色外观":"切换浅色外观",()=>{Capture();settings.LightTheme=!settings.LightTheme;LocalStore.Save(settings);Build();}));
        bottom.Children.Add(Text("第三方工具 · 非官方产品",12,false,true));
        var nav=new StackPanel();nav.Children.Add(Text("  桌面隐私",12,false,true));navButtons.Clear();
        string[] names=["shield","network","sliders-horizontal","clipboard-check"];
        for(int i=0;i<labels.Length;i++)
        {
            int n=i;var b=Button(labels[i],()=>{Capture();LocalStore.Save(settings);selectedPage=n;RenderPage();});
            var label=new StackPanel{Orientation=Orientation.Horizontal};label.Children.Add(Glyph(names[i],18));label.Children.Add(Text(labels[i],14));b.Content=label;b.HorizontalContentAlignment=HorizontalAlignment.Left;b.Margin=new Thickness(0,5,0,0);b.BorderThickness=new Thickness(0);nav.Children.Add(b);navButtons.Add(b);
        }
        side.Children.Add(nav);
        var main=new Grid();Grid.SetColumn(main,1);layout.Children.Add(main);
        main.RowDefinitions.Add(new(){Height=new GridLength(62)});main.RowDefinitions.Add(new(){Height=new GridLength(1,GridUnitType.Star)});main.RowDefinitions.Add(new(){Height=GridLength.Auto});
        var top=new DockPanel{Margin=new Thickness(36,0,36,0),VerticalAlignment=VerticalAlignment.Center};
        var tag=Text("Windows 桌面版",12,false,true);DockPanel.SetDock(tag,Dock.Right);top.Children.Add(tag);breadcrumb=Text("",13,false,true);top.Children.Add(breadcrumb);
        main.Children.Add(new Border{Child=top,BorderBrush=Brush("Line"),BorderThickness=new Thickness(0,0,0,1)});
        page=new StackPanel{Margin=new Thickness(36,30,36,24),MaxWidth=860};var scroll=new ScrollViewer{Content=page,VerticalScrollBarVisibility=ScrollBarVisibility.Auto,HorizontalScrollBarVisibility=ScrollBarVisibility.Disabled};Grid.SetRow(scroll,1);main.Children.Add(scroll);
        var footer=new DockPanel{Margin=new Thickness(36,16,36,18)};var footerBorder=new Border{Child=footer,BorderBrush=Brush("Line"),BorderThickness=new Thickness(0,1,0,0)};Grid.SetRow(footerBorder,2);main.Children.Add(footerBorder);
        actions=new StackPanel{Orientation=Orientation.Horizontal,VerticalAlignment=VerticalAlignment.Center};DockPanel.SetDock(actions,Dock.Right);footer.Children.Add(actions);
        status=Text("尚未开启防护",13,false,true);status.Margin=new Thickness(0,0,20,0);status.VerticalAlignment=VerticalAlignment.Center;footer.Children.Add(status);RenderPage();
    }
    private bool Applied()=>PolicyManager.Current() is {State:"Applied"};
    private void Verify()=>Run(async()=>{Capture();selectedPage=3;lastReport=null;RenderPage();status.Text="正在验证已启用的防护…";lastReport=await Verifier.RunAsync(new Progress<string>(s=>status.Text=s),CancellationToken.None);RenderPage();return lastReport.Summary;});
    private void RenderPage()
    {
        for(int i=0;i<navButtons.Count;i++){navButtons[i].Background=Brush(i==selectedPage?"Hover":"Sidebar");AutomationProperties.SetName(navButtons[i],labels[i]+(i==selectedPage?"，当前页面":""));}
        breadcrumb.Text="桌面隐私  /  "+labels[selectedPage];
        page.Children.Clear();sandboxHome=null!;claudePath=null!;proxyHost=null!;proxyPort=null!;proxyUser=null!;proxyPassword=null!;browsers=null!;managedTcp=null!;folders=null!;
        actions.Children.Clear();
        if(selectedPage==0)
        {
            actions.Children.Add(Button("验证防护",Verify));
            actions.Children.Add(Button(Applied()?"启动 Claude":"开启防护",()=>{if(Applied())Run(LaunchClaude);else Run(async()=>{Capture();LocalStore.Save(settings);await Task.Run(()=>PolicyManager.Apply(settings));lastReport=null;RenderPage();return "防护配置已应用，请验证实际效果。";});},true));
        }
        else if(selectedPage==3)actions.Children.Add(Button("验证防护",Verify,true));
        else actions.Children.Add(Button("返回隐私防护",()=>{Capture();LocalStore.Save(settings);selectedPage=0;RenderPage();}));
        status.Text=Applied()?"防护配置已应用 · 请通过本工具启动 Claude":"尚未开启防护";
        switch(selectedPage){case 0:PrivacyPage();break;case 1:NetworkPage();break;case 2:TelemetryPage();break;case 3:VerifyPage();break;}
    }
    private void Heading(string title,string description){page.Children.Add(Text(title,27,true));page.Children.Add(Note(description));}
    private void Card(Action content)
    {
        var outer=page;var body=new StackPanel();page=body;content();page=outer;
        outer.Children.Add(new Border{Child=body,Background=Brush("Panel"),BorderBrush=Brush("Line"),BorderThickness=new Thickness(1),CornerRadius=new CornerRadius(12),Padding=new Thickness(22,18,22,16),Margin=new Thickness(0,0,0,14)});
    }
    private void Expand(string title,Action content,bool open=false)
    {
        var outer=page;var body=new StackPanel{Margin=new Thickness(0,16,0,0)};page=body;content();page=outer;
        outer.Children.Add(new Expander{Header=title,Content=body,IsExpanded=open,Foreground=Brush("Text"),Margin=new Thickness(0,8,0,6)});
    }
    private void Divider()=>page.Children.Add(new Border{Height=1,Background=Brush("Line"),Margin=new Thickness(0,12,0,16)});
    private void PrivacyPage()
    {
        Heading("为 Claude 划清访问边界","选择要保护的本地资料，开启后验证实际效果。");
        Card(()=>{
            var row=new DockPanel();var badge=Text(Applied()?"配置已应用":"尚未开启",12,false,true);DockPanel.SetDock(badge,Dock.Right);row.Children.Add(badge);
            var title=new StackPanel{Orientation=Orientation.Horizontal};title.Children.Add(Logo(24));title.Children.Add(Text("Claude 桌面版",17,true));row.Children.Add(title);page.Children.Add(row);
            page.Children.Add(Text("从这里启动，让客户端在专属沙箱中运行。",13,false,true));
            Expand("安装与启动设置",()=>{
                sandboxHome=Field("Sandboxie 安装目录",settings.SandboxHome);var locate=new WrapPanel();locate.Children.Add(Button("选择目录",()=>{var d=new OpenFolderDialog();if(d.ShowDialog()==true)sandboxHome.Text=d.FolderName;}));locate.Children.Add(Button("下载 Sandboxie",()=>Open("https://sandboxie-plus.com/downloads/")));page.Children.Add(locate);
                claudePath=Field("Claude 程序路径",settings.ClaudePath);page.Children.Add(Button("选择 Claude 程序",()=>{var d=new OpenFileDialog{Filter="Windows 程序 (*.exe)|*.exe"};if(d.ShowDialog()==true)claudePath.Text=d.FileName;}));
                page.Children.Add(Text("可以先配置防护，安装 Claude 后再选择程序。",13,false,true));
            });
        });
        page.Children.Add(Text("本地资料",14,true));
        Card(()=>{
            browsers=new CheckBox{Content="保护浏览器资料",IsChecked=settings.ProtectBrowsers};page.Children.Add(browsers);
            page.Children.Add(Text("限制访问浏览器的配置、扩展和登录资料。",13,false,true));
            page.Children.Add(Text("已发现 "+PolicyManager.BrowserTargets().Count+" 个默认资料目录 · 仅对发现的目录应用规则",12,false,true));
            Divider();
            Expand("私人目录 · "+settings.PrivateFolders.Count+" 个",()=>{
                page.Children.Add(Text("添加不希望 Claude 访问的目录。",13,false,true));folders=new ListBox{MinHeight=60,MaxHeight=140};foreach(var folder in settings.PrivateFolders)folders.Items.Add(folder);page.Children.Add(folders);
                var row=new WrapPanel{Margin=new Thickness(0,10,0,0)};row.Children.Add(Button("添加目录",()=>{var d=new OpenFolderDialog();if(d.ShowDialog()==true&&!folders.Items.Contains(d.FolderName))folders.Items.Add(d.FolderName);}));row.Children.Add(Button("移除所选",()=>{if(folders.SelectedItem is {} item)folders.Items.Remove(item);}));page.Children.Add(row);
                page.Children.Add(Text("上传文件请使用未受限目录。不要添加 Claude 安装目录。",13,false,true));
            });
        });
        Card(()=>{
            var row=new DockPanel();var button=Button("配置连接",()=>{Capture();selectedPage=1;RenderPage();});DockPanel.SetDock(button,Dock.Right);row.Children.Add(button);
            var text=new StackPanel();text.Children.Add(Text("网络连接",14,true));text.Children.Add(Text(settings.NetworkMode==NetworkMode.ManagedTcp?"使用沙箱 TCP 代理 · 实验功能":"沿用当前网络与已有全局代理",13,false,true));row.Children.Add(text);page.Children.Add(row);
        });
        Expand("使用说明与恢复",()=>{
            page.Children.Add(Note("保留真实硬件信息，不改写系统语言。普通快捷方式启动的 Claude 不受本工具保护。修改已应用策略前，请先恢复限制再重新应用。"));
            page.Children.Add(Button("恢复访问限制",()=>{
                if(MessageBox.Show(this,"恢复后将撤销本工具的访问限制。请先退出受保护 Claude；沙箱数据不会被删除。","恢复访问限制",MessageBoxButton.OKCancel)!=MessageBoxResult.OK)return;
                Run(async()=>{await Task.Run(PolicyManager.Restore);lastReport=null;RenderPage();return "访问限制已恢复，沙箱数据保留。";});
            }));
        });
    }
    private void NetworkPage()
    {
        Heading("选择 Claude 的连接方式","已有全局代理时，保持当前网络即可。");
        Card(()=>{
            page.Children.Add(Text("本次检测出口",16,true));var ipValue=Text(currentIp,23,true);page.Children.Add(ipValue);
            page.Children.Add(Button("检测出口 IP",()=>Run(async()=>{currentIp=await NetworkProbe.ExitIpAsync(null,true,CancellationToken.None);ipValue.Text=currentIp;return "出口检测完成，仅代表本次请求。";})));
        });
        Card(()=>{
            managedTcp=new CheckBox{Content="使用住宅 SOCKS5 代理 · 实验功能",IsChecked=settings.NetworkMode==NetworkMode.ManagedTcp};page.Children.Add(managedTcp);
            page.Children.Add(Text("关闭时沿用当前网络；开启时仅接管沙箱内受支持的 TCP 连接。",13,false,true));
            var outer=page;var form=new StackPanel{Margin=new Thickness(0,16,0,0),Visibility=managedTcp.IsChecked==true?Visibility.Visible:Visibility.Collapsed};page=form;
            page.Children.Add(Button("粘贴识别代理",PasteProxy));
            proxyHost=Field("代理主机",settings.ProxyHost);proxyPort=Field("端口",settings.ProxyPort.ToString());
            Expand("用户名与密码（可选）",()=>{
                proxyUser=Field("用户名",settings.ProxyUser);page.Children.Add(Text("密码",13));proxyPassword=new PasswordBox{Margin=new Thickness(0,5,0,12)};AutomationProperties.SetName(proxyPassword,"代理密码");try{proxyPassword.Password=LocalStore.Unprotect(settings.ProtectedPassword);}catch{status.Text="旧代理凭证无法解密，请重新填写。";}page.Children.Add(proxyPassword);
                page.Children.Add(Text("凭证使用当前 Windows 用户加密保存。",12,false,true));
            },!string.IsNullOrEmpty(settings.ProxyUser));
            page.Children.Add(Button("保存并测试代理",()=>Run(async()=>{Capture();var proxy=NetworkProbe.Socks(settings.ProxyHost,settings.ProxyPort,settings.ProxyUser,LocalStore.Unprotect(settings.ProtectedPassword));LocalStore.Save(settings);return "代理请求出口："+await NetworkProbe.ExitIpAsync(proxy,false,CancellationToken.None)+"。仅测试连接，未接管 Claude。";}),true));
            page.Children.Add(Text("不接管 UDP / DNS，不提供全面防泄漏。重启 Windows 后需恢复并重新应用。",13,false,true));
            page=outer;page.Children.Add(form);managedTcp.Checked+=(s,e)=>form.Visibility=Visibility.Visible;managedTcp.Unchecked+=(s,e)=>form.Visibility=Visibility.Collapsed;
        });
        Card(()=>{
            page.Children.Add(Text("交叉检查 IP",15,true));page.Children.Add(Text("建议结合多个检测站点判断。风险评分不计入防护验证。",13,false,true));
            var links=new WrapPanel{Margin=new Thickness(0,10,0,0)};links.Children.Add(Button("ToDetect ↗",()=>Open("https://www.todetect.cn/")));links.Children.Add(Button("BrowserLeaks ↗",()=>Open("https://browserleaks.com/")));page.Children.Add(links);
        });
    }
    private void PasteProxy()
    {
        var value=Clipboard.GetText().Trim();if(!value.Contains("://"))value="socks5://"+value;
        if(!Uri.TryCreate(value,UriKind.Absolute,out var uri)||uri.Scheme!="socks5"||string.IsNullOrWhiteSpace(uri.Host)||uri.Port<1||uri.Port>65535||uri.AbsolutePath!="/"||uri.Query.Length>0||uri.Fragment.Length>0)throw new ArgumentException("请粘贴 socks5://用户名:密码@主机:端口，或 主机:端口。");
        proxyHost.Text=uri.Host;proxyPort.Text=uri.Port.ToString();var auth=uri.UserInfo.Split(':',2);proxyUser.Text=Uri.UnescapeDataString(auth[0]);proxyPassword.Password=auth.Length>1?Uri.UnescapeDataString(auth[1]):"";status.Text="代理信息已填入，尚未保存或连接。";
    }
    private void TelemetryPage()
    {
        Heading("管理可选数据使用","只调整可识别的官方偏好，保留登录与聊天连接。");
        Card(()=>{
            page.Children.Add(Text("模型改进",18,true));page.Children.Add(Note("关闭后，聊天不再用于模型改进。此偏好不等于关闭全部遥测，可能同步到其他设备。"));
            Divider();page.Children.Add(Text("操作前",13,true));page.Children.Add(Text("在 Claude 中打开 Settings → Privacy，保持模型改进开关可见。",14,false,true));
            var row=new WrapPanel{Margin=new Thickness(0,18,0,0)};row.Children.Add(Button("关闭模型改进",()=>Run(()=>Task.Run(TelemetrySettings.DisableOptional)),true));row.Children.Add(Button("打开隐私设置 ↗",()=>Open("https://claude.ai/settings/privacy")));page.Children.Add(row);
        });
        page.Children.Add(Note("识别并修改后会立即读回状态；只有实际修改的设置才会加入防护验证。"));
        Expand("恢复原有偏好",()=>{
            page.Children.Add(Button("恢复模型改进偏好",()=>{if(MessageBox.Show(this,"恢复会重新开启由本工具关闭的模型改进偏好，可能影响账户在其他设备上的设置。继续吗？","恢复账户偏好",MessageBoxButton.OKCancel)==MessageBoxResult.OK)Run(()=>Task.Run(TelemetrySettings.Restore));}));
        });
    }
    private void VerifyPage()
    {
        Heading("验证已开启的防护","用实际访问结果确认效果，只检查本工具已启用的保护项。");
        if(lastReport is null)
        {
            Card(()=>{
                var empty=new StackPanel{Margin=new Thickness(12,38,12,38),HorizontalAlignment=HorizontalAlignment.Center};var icon=Glyph("clipboard-check",32);icon.HorizontalAlignment=HorizontalAlignment.Center;icon.Margin=new Thickness(0,0,0,18);empty.Children.Add(icon);empty.Children.Add(Text("还没有本次验证结果",19,true));empty.Children.Add(Text("开启防护后，点击右下角“验证防护”。",13,false,true));page.Children.Add(empty);
            });
        }
        else
        {
            page.Children.Add(Text(lastReport.Summary,18,true));
            foreach(var check in lastReport.Checks)Card(()=>{
                page.Children.Add(Text(check.Status+"  ·  "+check.Name,15,true));page.Children.Add(Text(check.Evidence,13,false,true));
            });
            page.Children.Add(Button("导出本次报告",()=>{var d=new SaveFileDialog{Filter="JSON 报告|*.json",FileName="claudeFF-verification-"+DateTime.Now.ToString("yyyyMMdd-HHmm")+".json"};if(d.ShowDialog()==true){Json.Write(d.FileName,lastReport);status.Text="报告已导出。";}}));
        }
        Expand("验证范围与记录",()=>{
            page.Children.Add(Note("宿主可读、沙箱拒绝访问才算通过。资源不存在、接口错误或超时不算通过；不读取浏览器密码、Cookies 或私人文件内容。\n记录不代表捕获了 Claude 的全部访问。登录、聊天与附件等正常使用需在 Windows 实测，不计入防护验证。"));
            page.Children.Add(Button("打开本地记录",()=>{Directory.CreateDirectory(LocalStore.Root);Open(LocalStore.Root);}));
        });
    }
    private void Capture()
    {
        if(sandboxHome!=null)settings.SandboxHome=sandboxHome.Text.Trim();
        if(claudePath!=null)settings.ClaudePath=claudePath.Text.Trim();
        if(browsers!=null)settings.ProtectBrowsers=browsers.IsChecked==true;
        if(folders!=null)settings.PrivateFolders=folders.Items.Cast<string>().ToList();
        if(proxyHost!=null)
        {
            settings.NetworkMode=managedTcp?.IsChecked==true?NetworkMode.ManagedTcp:NetworkMode.Existing;
            settings.ProxyHost=proxyHost.Text.Trim();settings.ProxyUser=proxyUser.Text;
            if(int.TryParse(proxyPort.Text,out var port)&&port>=1&&port<=65535)settings.ProxyPort=port;
            else if(settings.NetworkMode==NetworkMode.ManagedTcp)throw new ArgumentException("端口必须为 1–65535。");
            settings.ProtectedPassword=LocalStore.Protect(proxyPassword.Password);
        }
    }
    private async Task<string> LaunchClaude()
    {
        Capture();LocalStore.Save(settings);
        var journal=PolicyManager.Current();if(journal is null||journal.State!="Applied")throw new InvalidOperationException("请先开启防护。");
        if(!File.Exists(settings.ClaudePath)||!settings.ClaudePath.EndsWith(".exe",StringComparison.OrdinalIgnoreCase))throw new InvalidOperationException("请在隐私防护 → 安装与启动设置中选择 Claude 程序。");
        using var api=new SandboxApi(journal.SandboxHome);
        foreach(var p in Process.GetProcessesByName(Path.GetFileNameWithoutExtension(settings.ClaudePath)))using(p)
            if(api.ProcessBox(p.Id)!=journal.Box)throw new InvalidOperationException("检测到沙箱外的同名客户端。请先退出它，避免启动请求转交到未受保护的实例。");
        using var start=api.Start(journal.Box,settings.ClaudePath);
        for(int i=0;i<30;i++)
        {
            await Task.Delay(300);
            foreach(var p in Process.GetProcessesByName(Path.GetFileNameWithoutExtension(settings.ClaudePath)))using(p)
                if(api.ProcessBox(p.Id)==journal.Box)return "已确认客户端进程进入专属沙箱。请测试登录、聊天和附件等正常功能。";
        }
        return "已发出沙箱启动请求，但未确认客户端进程；请检查安装方式和 Sandboxie 提示。未记为受保护启动成功。";
    }
    private async void Run(Func<Task<string>> action)
    {
        if(busy)return;busy=true;layout.IsEnabled=false;status.Text="正在处理，请稍候…";
        try{status.Text=await action();}
        catch(Exception ex){status.Text="操作未完成："+LocalStore.HidePath(ex.Message);}
        finally{busy=false;layout.IsEnabled=true;}
    }
    private TextBox Field(string label,string value)
    {
        page.Children.Add(Text(label,13));var box=new TextBox{Text=value,Margin=new Thickness(0,5,0,12)};AutomationProperties.SetName(box,label);page.Children.Add(box);return box;
    }
    private TextBlock Text(string value,double size=14,bool bold=false,bool muted=false)=>new(){Text=value,FontSize=size,FontWeight=bold?FontWeights.SemiBold:FontWeights.Normal,Foreground=Brush(muted?"Muted":"Text"),TextWrapping=TextWrapping.Wrap,Margin=new Thickness(0,0,0,6),LineHeight=size*1.55};
    private TextBlock Note(string value){var t=Text(value,13,false,true);t.Margin=new Thickness(0,8,0,18);return t;}
    private Button Button(string text,Action action,bool primary=false)
    {
        var button=new Button{Content=text};if(primary){button.Background=Brush("Button");button.Foreground=Brush("OnButton");}button.Click+=(s,e)=>{try{action();}catch(Exception ex){status.Text="操作未完成："+LocalStore.HidePath(ex.Message);}};return button;
    }
    private static void Open(string target)=>Process.Start(new ProcessStartInfo(target){UseShellExecute=true});
}

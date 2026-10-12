using ClaudeFF.Core;
int count=0;
void Test(string name,Action check){check();count++;Console.WriteLine("PASS "+name);}
void Assert(bool value){if(!value)throw new Exception("Assertion failed");}
void Throws(Action action){try{action();}catch(ArgumentException){return;}throw new Exception("Expected rejection");}
Test("缺少验证项不能通过",()=>Assert(!new Report().RequiredPassed));
Test("必需项未知不能通过",()=>Assert(!new Report{Checks=[new("a","a",Verdict.Unknown,"",true)]}.RequiredPassed));
Test("失败不能被其他成功覆盖",()=>Assert(!new Report{Checks=[new("a","a",Verdict.Pass,"",true),new("b","b",Verdict.Fail,"",true)]}.RequiredPassed));
Test("非必需信息不影响已开启防护结论",()=>Assert(new Report{Checks=[new("a","a",Verdict.Pass,"",true),new("b","b",Verdict.Unknown,"")]}.RequiredPassed));
var baseline=new AccessSample("文件","C:\\test\\file","readable");
Test("确切拒绝才通过",()=>Assert(VerificationRules.FileDenied("a",baseline,baseline with{Outcome="denied"}).Verdict==Verdict.Pass));
Test("文件不存在不能冒充拦截",()=>Assert(VerificationRules.FileDenied("a",baseline,baseline with{Outcome="missing"}).Verdict==Verdict.Unknown));
Test("宿主本身无权限不能冒充沙箱保护",()=>Assert(VerificationRules.FileDenied("a",baseline with{Outcome="denied"},baseline with{Outcome="denied"}).Verdict==Verdict.Unknown));
Test("未生成结果不能通过",()=>Assert(VerificationRules.FileDenied("a",baseline,null).Verdict==Verdict.Unknown));
Test("沙箱仍可读必须失败",()=>Assert(VerificationRules.FileDenied("a",baseline,baseline).Verdict==Verdict.Fail));
Test("规则拒绝换行注入",()=>Throws(()=>VerificationRules.FolderRule("C:\\Secret\nOpenFilePath=*")));
Test("规则拒绝通配符",()=>Throws(()=>VerificationRules.FolderRule("C:\\Users\\*")));
Test("规则拒绝整盘与相对路径",()=>{Throws(()=>VerificationRules.FolderRule("C:\\"));Throws(()=>VerificationRules.FolderRule("C:\\Users\\..\\Windows"));});
Test("目录规则带路径边界",()=>Assert(VerificationRules.FolderRule("C:\\Secret")=="C:\\Secret\\"));
Test("代理拒绝凭证/路径注入",()=>Throws(()=>NetworkProbe.Socks("user@host",1080,"","")));
Test("代理拒绝错误端口",()=>Throws(()=>NetworkProbe.Socks("localhost",0,"","")));
Test("SOCKS 不把凭证嵌入 URL",()=>{var p=NetworkProbe.Socks("127.0.0.1",1080,"alice","secret");Assert(!p.Address!.ToString().Contains("secret"));Assert(p.Address.Scheme=="socks5");});
Test("报告保存保留逐项状态",()=>{var path=Path.Combine(Path.GetTempPath(),Guid.NewGuid()+".json");try{Json.Write(path,new Report{Checks=[new("a","测试",Verdict.Unknown,"未验证",true)]});Assert(Json.Read<Report>(path).Checks[0].Verdict==Verdict.Unknown);}finally{File.Delete(path);}});
Test("恢复不覆盖外部更改",()=>Assert(!VerificationRules.CanRestore(["user-edit"],["old"],["new"],true)));
Test("未完成的追加可以回滚",()=>Assert(VerificationRules.CanRestore(["a"],[],["a","b"],false)));
Test("已完成配置被删减需保留外部编辑",()=>Assert(!VerificationRules.CanRestore(["a"],[],["a","b"],true)));
Test("已恢复的键可重复恢复",()=>Assert(VerificationRules.CanRestore(["old"],["old"],["new"],true)));
await SocksTests.Run();

Console.WriteLine($"{count} decision checks and 3 SOCKS integration checks passed.");

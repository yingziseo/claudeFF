using System.Windows;
using System.Windows.Controls;
using System.Windows.Media;
namespace ClaudeFF.App;
internal static class Theme
{
    public static void Apply(bool light)
    {
        var r=Application.Current.Resources;
        var colors=new Dictionary<string,string> {
            ["Bg"]=light?"#FAFAF9":"#18191B",["Sidebar"]=light?"#F1F1EF":"#202123",
            ["Panel"]=light?"#FFFFFF":"#242527",["Inset"]=light?"#F7F7F5":"#1B1C1E",
            ["Text"]=light?"#292927":"#EEEDE9",["Muted"]=light?"#686965":"#ACAAA5",
            ["Line"]=light?"#E4E4DF":"#37383A",["Hover"]=light?"#E9E9E5":"#303133",
            ["Accent"]="#D97757",["Button"]=light?"#292927":"#ECEBE7",["OnButton"]=light?"#FFFFFF":"#232320"
        };
        foreach(var (name,color) in colors)r[name]=new SolidColorBrush((Color)ColorConverter.ConvertFromString(color));
        var control=new Style(typeof(Control));control.Setters.Add(new Setter(Control.FontFamilyProperty,new FontFamily("Segoe UI, Microsoft YaHei UI")));control.Setters.Add(new Setter(Control.FontSizeProperty,14d));control.Setters.Add(new Setter(Control.ForegroundProperty,r["Text"]));r[typeof(Control)]=control;
        var text=new Style(typeof(TextBlock));text.Setters.Add(new Setter(TextBlock.ForegroundProperty,r["Text"]));text.Setters.Add(new Setter(TextBlock.FontSizeProperty,14d));text.Setters.Add(new Setter(TextBlock.TextWrappingProperty,TextWrapping.Wrap));r[typeof(TextBlock)]=text;
        foreach(var type in new[]{typeof(TextBox),typeof(PasswordBox),typeof(ListBox)})
        {
            var style=new Style(type,control);style.Setters.Add(new Setter(Control.BackgroundProperty,r["Panel"]));style.Setters.Add(new Setter(Control.BorderBrushProperty,r["Line"]));style.Setters.Add(new Setter(Control.PaddingProperty,new Thickness(10,8,10,8)));if(type!=typeof(ListBox))style.Setters.Add(new Setter(Control.TemplateProperty,System.Windows.Markup.XamlReader.Parse("""
<ControlTemplate xmlns="http://schemas.microsoft.com/winfx/2006/xaml/presentation" xmlns:x="http://schemas.microsoft.com/winfx/2006/xaml">
<Border x:Name="edge" Background="{TemplateBinding Background}" BorderBrush="{TemplateBinding BorderBrush}" BorderThickness="1" CornerRadius="8"><ScrollViewer x:Name="PART_ContentHost" Margin="{TemplateBinding Padding}"/></Border>
<ControlTemplate.Triggers><Trigger Property="IsKeyboardFocusWithin" Value="True"><Setter TargetName="edge" Property="BorderBrush" Value="{DynamicResource Accent}"/></Trigger></ControlTemplate.Triggers>
</ControlTemplate>
""")));r[type]=style;
        }
        var button=new Style(typeof(Button),control);button.Setters.Add(new Setter(Control.HorizontalContentAlignmentProperty,HorizontalAlignment.Center));button.Setters.Add(new Setter(Control.BorderThicknessProperty,new Thickness(1)));button.Setters.Add(new Setter(Control.PaddingProperty,new Thickness(17,10,17,10)));button.Setters.Add(new Setter(Control.BackgroundProperty,r["Panel"]));button.Setters.Add(new Setter(Control.BorderBrushProperty,r["Line"]));button.Setters.Add(new Setter(FrameworkElement.MarginProperty,new Thickness(0,0,8,0)));
        var template=new ControlTemplate(typeof(Button));var border=new FrameworkElementFactory(typeof(Border));border.SetValue(Border.CornerRadiusProperty,new CornerRadius(8));border.SetBinding(Border.BackgroundProperty,new System.Windows.Data.Binding("Background"){RelativeSource=new(System.Windows.Data.RelativeSourceMode.TemplatedParent)});border.SetBinding(Border.BorderBrushProperty,new System.Windows.Data.Binding("BorderBrush"){RelativeSource=new(System.Windows.Data.RelativeSourceMode.TemplatedParent)});border.SetBinding(Border.BorderThicknessProperty,new System.Windows.Data.Binding("BorderThickness"){RelativeSource=new(System.Windows.Data.RelativeSourceMode.TemplatedParent)});
        var content=new FrameworkElementFactory(typeof(ContentPresenter));content.SetValue(FrameworkElement.MarginProperty,new Thickness(16,10,16,10));content.SetBinding(FrameworkElement.HorizontalAlignmentProperty,new System.Windows.Data.Binding("HorizontalContentAlignment"){RelativeSource=new(System.Windows.Data.RelativeSourceMode.TemplatedParent)});content.SetValue(FrameworkElement.VerticalAlignmentProperty,VerticalAlignment.Center);border.AppendChild(content);template.VisualTree=border;button.Setters.Add(new Setter(Control.TemplateProperty,template));
        var hover=new Trigger{Property=UIElement.IsMouseOverProperty,Value=true};hover.Setters.Add(new Setter(Control.BorderBrushProperty,r["Accent"]));button.Triggers.Add(hover);
        var focus=new Trigger{Property=UIElement.IsKeyboardFocusedProperty,Value=true};focus.Setters.Add(new Setter(Control.BorderBrushProperty,r["Accent"]));focus.Setters.Add(new Setter(Control.BorderThicknessProperty,new Thickness(2)));button.Triggers.Add(focus);
        var disabled=new Trigger{Property=UIElement.IsEnabledProperty,Value=false};disabled.Setters.Add(new Setter(UIElement.OpacityProperty,.45));button.Triggers.Add(disabled);r[typeof(Button)]=button;
        var check=new Style(typeof(CheckBox),control);
        check.Setters.Add(new Setter(FrameworkElement.MarginProperty,new Thickness(0,2,0,10)));
        check.Setters.Add(new Setter(Control.TemplateProperty,System.Windows.Markup.XamlReader.Parse("""
<ControlTemplate xmlns="http://schemas.microsoft.com/winfx/2006/xaml/presentation" xmlns:x="http://schemas.microsoft.com/winfx/2006/xaml" TargetType="CheckBox">
<Grid Background="Transparent" MinHeight="32"><Grid.ColumnDefinitions><ColumnDefinition/><ColumnDefinition Width="Auto"/></Grid.ColumnDefinitions>
<ContentPresenter VerticalAlignment="Center" Margin="0,0,18,0" RecognizesAccessKey="True"/>
<Border x:Name="track" Grid.Column="1" Width="36" Height="22" CornerRadius="11" Background="{DynamicResource Line}"><Ellipse x:Name="thumb" Width="16" Height="16" Fill="{DynamicResource Muted}" HorizontalAlignment="Left" Margin="3"/></Border></Grid>
<ControlTemplate.Triggers>
<Trigger Property="IsChecked" Value="True"><Setter TargetName="track" Property="Background" Value="{DynamicResource Button}"/><Setter TargetName="thumb" Property="Fill" Value="{DynamicResource OnButton}"/><Setter TargetName="thumb" Property="HorizontalAlignment" Value="Right"/></Trigger>
<Trigger Property="IsKeyboardFocused" Value="True"><Setter TargetName="track" Property="BorderBrush" Value="{DynamicResource Accent}"/><Setter TargetName="track" Property="BorderThickness" Value="2"/></Trigger>
<Trigger Property="IsEnabled" Value="False"><Setter Property="Opacity" Value="0.5"/></Trigger>
</ControlTemplate.Triggers></ControlTemplate>
""")));
        r[typeof(CheckBox)]=check;
    }
}

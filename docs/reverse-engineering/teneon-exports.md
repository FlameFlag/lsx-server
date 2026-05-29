# TeneonIERelease.dll Demangled Export Map

Source: `decompiled/local/lt2_install/TeneonIERelease.dll`

Ghidra analyzers used:

- `Demangler Microsoft`
- `Windows x86 PE Exception Handling`
- `Windows x86 RTTI Analyzer`
- `PDB Universal` attempted, but no PDB was available locally.

| VA           | Ordinal | Decorated export                                                            | Demangled meaning                                              |
| ------------ | ------- | --------------------------------------------------------------------------- | -------------------------------------------------------------- |
| `0x10001020` | 1       | `??0IEBrowserContainer@@QAE@PAUHWND__@@@Z`                                  | `IEBrowserContainer::IEBrowserContainer(HWND__ *)`             |
| `0x10001180` | 2       | `??1IEBrowserContainer@@QAE@XZ`                                             | `IEBrowserContainer::~IEBrowserContainer()`                    |
| `0x10001210` | 9       | `?LoadURL@IEBrowserContainer@@...`                                          | `IEBrowserContainer::LoadURL(std::basic_string<char> const &)` |
| `0x10001310` | 11      | `?Refresh@IEBrowserContainer@@UAE?AW4Results@1@XZ`                          | `IEBrowserContainer::Refresh()`                                |
| `0x10001350` | 21      | `?UpdateWindow@IEBrowserContainer@@UAEXXZ`                                  | `IEBrowserContainer::UpdateWindow()`                           |
| `0x10001380` | 12      | `?Resize@IEBrowserContainer@@UAEXHHHH@Z`                                    | `IEBrowserContainer::Resize(int, int, int, int)`               |
| `0x100013F0` | 13      | `?Resize@IEBrowserContainer@@UAEXUtagRECT@@@Z`                              | `IEBrowserContainer::Resize(tagRECT)`                          |
| `0x10001460` | 14      | `?ResizeBrowser@IEBrowserContainer@@IAEXHHHH@Z`                             | `IEBrowserContainer::ResizeBrowser(int, int, int, int)`        |
| `0x100014C0` | 15      | `?ResizeBrowser@IEBrowserContainer@@IAEXUtagRECT@@@Z`                       | `IEBrowserContainer::ResizeBrowser(tagRECT)`                   |
| `0x10001520` | 16      | `?ResizeToClientArea@IEBrowserContainer@@UAEXXZ`                            | `IEBrowserContainer::ResizeToClientArea()`                     |
| `0x10001570` | 8       | `?Hide@IEBrowserContainer@@UAEXXZ`                                          | `IEBrowserContainer::Hide()`                                   |
| `0x100015A0` | 17      | `?Show@IEBrowserContainer@@UAEXXZ`                                          | `IEBrowserContainer::Show()`                                   |
| `0x100015D0` | 19      | `?Stop@IEBrowserContainer@@UAEXXZ`                                          | `IEBrowserContainer::Stop()`                                   |
| `0x100015E0` | 3       | `?Back@IEBrowserContainer@@UAE_NXZ`                                         | `IEBrowserContainer::Back()`                                   |
| `0x100015F0` | 6       | `?Forward@IEBrowserContainer@@UAE_NXZ`                                      | `IEBrowserContainer::Forward()`                                |
| `0x10001600` | 18      | `?StatusText@IEBrowserContainer@@...`                                       | `IEBrowserContainer::StatusText()`                             |
| `0x10001780` | 20      | `?TitleText@IEBrowserContainer@@...`                                        | `IEBrowserContainer::TitleText()`                              |
| `0x10001900` | 7       | `?GetURL@IEBrowserContainer@@...`                                           | `IEBrowserContainer::GetURL()`                                 |
| `0x10001A80` | 10      | `?Print@IEBrowserContainer@@UAEX_N@Z`                                       | `IEBrowserContainer::Print(bool)`                              |
| `0x10001CB0` | 5       | `?Create@CWebBrowser@@UAEHPBD0KABUtagRECT@@PAVCWnd@@IPAUCCreateContext@@@Z` | `CWebBrowser::Create(...)`                                     |
| `0x10001CE0` | 4       | `?Create@CWebBrowser@@QAEHPBDKABUtagRECT@@PAVCWnd@@IPAVCFile@@HPAG@Z`       | `CWebBrowser::Create(...)`                                     |

Important behavior already decompiled in
`decompiled/src/browser/teneon_ie_browser.c`:

- `IEBrowserContainer::LoadURL` calls the hosted browser's `Navigate2`-style vtable slot with URL only.
- Post data and custom headers are zero.
- This confirms the browser-side LSX check is ordinary GET navigation.

//go:build windows

package main

import (
	"encoding/hex"
	"fmt"
	"math"
	"reflect"
	"strings"
	"sync/atomic"
	"syscall"
	"unsafe"

	"github.com/jchv/go-webview2/webviewloader"
	"golang.org/x/sys/windows"
)

//nolint:gochecknoglobals,lll // IIDs COM: nomes fixos + alinhamento gofmt = 124 col.
var (
	IIDICoreWebView2AddScriptToExecuteOnDocumentCreatedCompletedHandler     = NewGUID("{B99369F3-9B11-47B5-BC6F-8E7895FCEA17}")
	IIDICoreWebView2ExecuteScriptCompletedHandler                           = NewGUID("{49511172-CC67-4BCA-9923-137112F4C4CC}")
	IIDICoreWebView2CapturePreviewCompletedHandler                          = NewGUID("{697E05E9-3D8F-45FA-96F4-8FFE1EDEDAF5}")
	IIDICoreWebView2CallDevToolsProtocolMethodCompletedHandler              = NewGUID("{5C4889F0-5EF6-4C5A-952C-D8F1B92D0574}")
	IIDICoreWebView2ContainsFullScreenElementChangedEventHandler            = NewGUID("{E45D98B1-AFEF-45BE-8BAF-6C7728867F73}")
	IIDICoreWebView2ContentLoadingEventHandler                              = NewGUID("{364471E7-F2BE-4910-BDBA-D72077D51C4B}")
	IIDICoreWebView2DocumentTitleChangedEventHandler                        = NewGUID("{F5F2B923-953E-4042-9F95-F3A118E1AFD4}")
	IIDICoreWebView2HistoryChangedEventHandler                              = NewGUID("{C79A420C-EFD9-4058-9295-3E8B4BCAB645}")
	IIDICoreWebView2NavigationCompletedEventHandler                         = NewGUID("{D33A35BF-1C49-4F98-93AB-006E0533FE1C}")
	IIDICoreWebView2NavigationStartingEventHandler                          = NewGUID("{9ADBE429-F36D-432B-9DDC-F8881FBD76E3}")
	IIDICoreWebView2NewWindowRequestedEventHandler                          = NewGUID("{D4C185FE-C81C-4989-97AF-2D3FA7AB5651}")
	IIDICoreWebView2PermissionRequestedEventHandler                         = NewGUID("{15E1C6A3-C72A-4DF3-91D7-D097FBEC6BFD}")
	IIDICoreWebView2ProcessFailedEventHandler                               = NewGUID("{79E0AEA4-990B-42D9-AA1D-0FCC2E5BC7F1}")
	IIDICoreWebView2ScriptDialogOpeningEventHandler                         = NewGUID("{EF381BF9-AFA8-4E37-91C4-8AC48524BDFB}")
	IIDICoreWebView2SourceChangedEventHandler                               = NewGUID("{3C067F9F-5388-4772-8B48-79F7EF1AB37C}")
	IIDICoreWebView2WebMessageReceivedEventHandler                          = NewGUID("{57213F19-00E6-49FA-8E07-898EA01ECBD2}")
	IIDICoreWebView2WebResourceRequestedEventHandler                        = NewGUID("{AB00B74C-15F1-4646-80E8-E76341D25D71}")
	IIDICoreWebView2WindowCloseRequestedEventHandler                        = NewGUID("{5C19E9E0-092F-486B-AFFA-CA8231913039}")
	IIDICoreWebView2DOMContentLoadedEventHandler                            = NewGUID("{4BAC7E9C-199E-49ED-87ED-249303ACF019}")
	IIDICoreWebView2WebResourceResponseReceivedEventHandler                 = NewGUID("{7DE9898A-24F5-40C3-A2DE-D4F458E69828}")
	IIDICoreWebView2TrySuspendCompletedHandler                              = NewGUID("{00F206A7-9D17-4605-91F6-4E8E4DE192E3}")
	IIDICoreWebView2DownloadStartingEventHandler                            = NewGUID("{EFEDC989-C396-41CA-83F7-07F845A55724}")
	IIDICoreWebView2FrameCreatedEventHandler                                = NewGUID("{38059770-9BAA-11EB-A8B3-0242AC130003}")
	IIDICoreWebView2IsDocumentPlayingAudioChangedEventHandler               = NewGUID("{5DEF109A-2F4B-49FA-B7F6-11C39E513328}")
	IIDICoreWebView2IsMutedChangedEventHandler                              = NewGUID("{57D90347-CD0E-4952-A4A2-7483A2756F08}")
	IIDICoreWebView2IsDefaultDownloadDialogOpenChangedEventHandler          = NewGUID("{3117DA26-AE13-438D-BD46-EDBEB2C4CE81}")
	IIDICoreWebView2ContextMenuRequestedEventHandler                        = NewGUID("{04D3FE1D-AB87-42FB-A898-DA241D35B63C}")
	IIDICoreWebView2StatusBarTextChangedEventHandler                        = NewGUID("{A5E3B0D0-10DF-4156-BFAD-3B43867ACAC6}")
	IIDICoreWebView2GetFaviconCompletedHandler                              = NewGUID("{A2508329-7DA8-49D7-8C05-FA125E4AEE8D}")
	IIDICoreWebView2FaviconChangedEventHandler                              = NewGUID("{2913DA94-833D-4DE0-8DCA-900FC524A1A4}")
	IIDICoreWebView2PrintCompletedHandler                                   = NewGUID("{8FD80075-ED08-42DB-8570-F5D14977461E}")
	IIDICoreWebView2PrintToPdfStreamCompletedHandler                        = NewGUID("{4C9F8229-8F93-444F-A711-2C0DFD6359D5}")
	IIDICoreWebView2CursorChangedEventHandler                               = NewGUID("{9DA43CCC-26E1-4DAD-B56C-D8961C94C571}")
	IIDICoreWebView2NonClientRegionChangedEventHandler                      = NewGUID("{4A794E66-AA6C-46BD-93A3-382196837680}")
	IIDICoreWebView2RegionRectCollectionView                                = NewGUID("{333353B8-48BF-4449-8FCC-22697FAF5753}")
	IIDICoreWebView2AcceleratorKeyPressedEventHandler                       = NewGUID("{B29C7E28-FA79-41A8-8E44-65811C76DCB2}")
	IIDICoreWebView2FocusChangedEventHandler                                = NewGUID("{05EA24BD-6452-4926-9014-4B82B498135D}")
	IIDICoreWebView2MoveFocusRequestedEventHandler                          = NewGUID("{69035451-6DC7-4CB8-9BCE-B2BD70AD289F}")
	IIDICoreWebView2ZoomFactorChangedEventHandler                           = NewGUID("{B52D71D6-C4DF-4543-A90C-64A3E60F38CB}")
	IIDICoreWebView2RasterizationScaleChangedEventHandler                   = NewGUID("{9C98C8B1-AC53-427E-A345-3049B5524BBE}")
	IIDICoreWebView2CreateCoreWebView2ControllerCompletedHandler            = NewGUID("{6C4819F3-C9B7-4260-8127-C9F5BDE7F68C}")
	IIDICoreWebView2NewBrowserVersionAvailableEventHandler                  = NewGUID("{F9A2976E-D34E-44FC-ADEE-81B6B57CA914}")
	IIDICoreWebView2BrowserExtensionRemoveCompletedHandler                  = NewGUID("{8E41909A-9B18-4BB1-8CDF-930F467A50BE}")
	IIDICoreWebView2BrowserExtensionEnableCompletedHandler                  = NewGUID("{30C186CE-7FAD-421F-A3BC-A8EAF071DDB8}")
	IIDICoreWebView2StringCollection                                        = NewGUID("{F41F3F8A-BCC3-11EB-8529-0242AC130003}")
	IIDICoreWebView2ClientCertificateCollection                             = NewGUID("{EF5674D2-BCC3-11EB-8529-0242AC130003}")
	IIDICoreWebView2CustomItemSelectedEventHandler                          = NewGUID("{49E1D0BC-FE9E-4481-B7C2-32324AA21998}")
	IIDICoreWebView2GetCookiesCompletedHandler                              = NewGUID("{5A4F5069-5C15-47C3-8646-F4DE1C116670}")
	IIDICoreWebView2CookieList                                              = NewGUID("{F7F6F714-5D2A-43C6-9503-346ECE02D186}")
	IIDICoreWebView2BytesReceivedChangedEventHandler                        = NewGUID("{828E8AB6-D94C-4264-9CEF-5217170D6251}")
	IIDICoreWebView2EstimatedEndTimeChangedEventHandler                     = NewGUID("{28F0D425-93FE-4E63-9F8D-2AEEC6D3BA1E}")
	IIDICoreWebView2StateChangedEventHandler                                = NewGUID("{81336594-7EDE-4BA9-BF71-ACF0A95B58DD}")
	IIDICoreWebView2FrameDestroyedEventHandler                              = NewGUID("{59DD7B4C-9BAA-11EB-A8B3-0242AC130003}")
	IIDICoreWebView2FrameNameChangedEventHandler                            = NewGUID("{435C7DC8-9BAA-11EB-A8B3-0242AC130003}")
	IIDICoreWebView2FrameContentLoadingEventHandler                         = NewGUID("{0D6156F2-D332-49A7-9E03-7D8F2FEEEE54}")
	IIDICoreWebView2FrameDOMContentLoadedEventHandler                       = NewGUID("{38D9520D-340F-4D1E-A775-43FCE9753683}")
	IIDICoreWebView2FrameNavigationCompletedEventHandler                    = NewGUID("{609302AD-0E36-4F9A-A210-6A45272842A9}")
	IIDICoreWebView2FrameNavigationStartingEventHandler                     = NewGUID("{E79908BF-2D5D-4968-83DB-263FEA2C1DA3}")
	IIDICoreWebView2FrameWebMessageReceivedEventHandler                     = NewGUID("{E371E005-6D1D-4517-934B-A8F1629C62A5}")
	IIDICoreWebView2ProfileDeletedEventHandler                              = NewGUID("{DF35055D-772E-4DBE-B743-5FBF74A2B258}")
	IIDICoreWebView2WebResourceResponseViewGetContentCompletedHandler       = NewGUID("{875738E1-9FA2-40E3-8B74-2E8972DD6FE7}")
	IIDICoreWebView2NotificationCloseRequestedEventHandler                  = NewGUID("{47C32D23-1E94-4733-85F1-D9BF4ACD0974}")
	IIDICoreWebView2                                                        = NewGUID("{76ECEACB-0462-4D94-AC83-423A6793775E}")
	IIDICoreWebView2_2                                                      = NewGUID("{9E8F0CF8-E670-4B5E-B2BC-73E061E3184C}")
	IIDICoreWebView2_3                                                      = NewGUID("{A0D6DF20-3B92-416D-AA0C-437A9C727857}")
	IIDICoreWebView2_4                                                      = NewGUID("{20D02D59-6DF2-42DC-BD06-F98A694B1302}")
	IIDICoreWebView2_8                                                      = NewGUID("{E9632730-6E1E-43AB-B7B8-7B2C9E62E094}")
	IIDICoreWebView2_9                                                      = NewGUID("{4D7B2EAB-9FDC-468D-B998-A9260B5ED651}")
	IIDICoreWebView2_11                                                     = NewGUID("{0BE78E56-C193-4051-B943-23B460C08BDB}")
	IIDICoreWebView2_12                                                     = NewGUID("{35D69927-BCFA-4566-9349-6B3E0D154CAC}")
	IIDICoreWebView2_15                                                     = NewGUID("{517B2D1D-7DAE-4A66-A4F4-10352FFB9518}")
	IIDICoreWebView2_16                                                     = NewGUID("{0EB34DC9-9F91-41E1-8639-95CD5943906B}")
	IIDICoreWebView2Settings                                                = NewGUID("{E562E4F0-D7FA-43AC-8D71-C05150499F00}")
	IIDICoreWebView2Settings4                                               = NewGUID("{CB56846C-4168-4D53-B04F-03B6D6796FF2}")
	IIDICoreWebView2Controller                                              = NewGUID("{4D00C0D1-9434-4EB6-8078-8697A560334F}")
	IIDICoreWebView2Controller3                                             = NewGUID("{F9614724-5D2B-41DC-AEF7-73D62B51543B}")
	IIDICoreWebView2Environment                                             = NewGUID("{B96D755E-0319-4E92-A296-23436F46A1FC}")
	IIDICoreWebView2EnvironmentOptions                                      = NewGUID("{2FDE08A8-1E9A-4766-8C05-95A9CEB9D1C5}")
	IIDICoreWebView2AcceleratorKeyPressedEventArgs                          = NewGUID("{9F760F8A-FB79-42BE-9990-7B56900FA9C7}")
	IIDICoreWebView2BasicAuthenticationRequestedEventArgs                   = NewGUID("{EF05516F-D897-4F9E-B672-D8E2307A3FB0}")
	IIDICoreWebView2BasicAuthenticationResponse                             = NewGUID("{07023F7D-2D77-4D67-9040-6E7D428C6A40}")
	IIDICoreWebView2BrowserExtension                                        = NewGUID("{7EF7FFA0-FAC5-462C-B189-3D9EDBE575DA}")
	IIDICoreWebView2Certificate                                             = NewGUID("{C5FB2FCE-1CAC-4AEE-9C79-5ED0362EAAE0}")
	IIDICoreWebView2ClientCertificate                                       = NewGUID("{E7188076-BCC3-11EB-8529-0242AC130003}")
	IIDICoreWebView2ClientCertificateRequestedEventArgs                     = NewGUID("{BC59DB28-BCC3-11EB-8529-0242AC130003}")
	IIDICoreWebView2CompositionController                                   = NewGUID("{3DF9B733-B9AE-4A15-86B4-EB9EE9826469}")
	IIDICoreWebView2CompositionController3                                  = NewGUID("{9570570E-4D76-4361-9EE1-F04D0DBDFB1E}")
	IIDICoreWebView2CompositionController4                                  = NewGUID("{7C367B9B-3D2B-450F-9E58-D61A20F486AA}")
	IIDICoreWebView2ContextMenuItem                                         = NewGUID("{7AED49E3-A93F-497A-811C-749C6B6B6C65}")
	IIDICoreWebView2ContextMenuTarget                                       = NewGUID("{B8611D99-EED6-4F3F-902C-A198502AD472}")
	IIDICoreWebView2ContextMenuRequestedEventArgs                           = NewGUID("{A1D309EE-C03F-11EB-8529-0242AC130003}")
	IIDICoreWebView2ControllerOptions                                       = NewGUID("{12AAE616-8CCB-44EC-BCB3-EB1831881635}")
	IIDICoreWebView2Cookie                                                  = NewGUID("{AD26D6BE-1486-43E6-BF87-A2034006CA21}")
	IIDICoreWebView2CookieManager                                           = NewGUID("{177CD9E7-B6F5-451A-94A0-5D7A3A4C4141}")
	IIDICoreWebView2CustomSchemeRegistration                                = NewGUID("{D60AC92C-37A6-4B26-A39E-95CFE59047BB}")
	IIDICoreWebView2DownloadOperation                                       = NewGUID("{3D6B6CF2-AFE1-44C7-A995-C65117714336}")
	IIDICoreWebView2DownloadStartingEventArgs                               = NewGUID("{E99BBE21-43E9-4544-A732-282764EAFA60}")
	IIDICoreWebView2ExecuteScriptResult                                     = NewGUID("{0CE15963-3698-4DF7-9399-71ED6CDD8C9F}")
	IIDICoreWebView2Frame                                                   = NewGUID("{F1131A5E-9BA9-11EB-A8B3-0242AC130003}")
	IIDICoreWebView2Frame2                                                  = NewGUID("{7A6A5834-D185-4DBF-B63F-4A9BC43107D4}")
	IIDICoreWebView2FrameInfo2                                              = NewGUID("{56F85CFA-72C4-11EE-B962-0242AC120002}")
	IIDICoreWebView2HttpHeadersCollectionIterator                           = NewGUID("{0702FC30-F43B-47BB-AB52-A42CB552AD9F}")
	IIDICoreWebView2LaunchingExternalURISchemeEventArgs                     = NewGUID("{07D1A6C3-7175-4BA1-9306-E593CA07E46C}")
	IIDICoreWebView2MoveFocusRequestedEventArgs                             = NewGUID("{2D6AA13B-3839-4A15-92FC-D88B3C0D9C9D}")
	IIDICoreWebView2NavigationCompletedEventArgs                            = NewGUID("{30D68B7D-20D9-4752-A9CA-EC8448FBB5C1}")
	IIDICoreWebView2NavigationStartingEventArgs                             = NewGUID("{5B495469-E119-438A-9B18-7604F25F2E49}")
	IIDICoreWebView2NewWindowRequestedEventArgs                             = NewGUID("{34ACB11C-FC37-4418-9132-F9C21D1EAFB9}")
	IIDICoreWebView2PermissionRequestedEventArgs                            = NewGUID("{973AE2EF-FF18-4894-8FB2-3C758F046810}")
	IIDICoreWebView2PermissionSettingCollectionView                         = NewGUID("{F5596F62-3DE5-47B1-91E8-A4104B596B96}")
	IIDICoreWebView2PointerInfo                                             = NewGUID("{E6995887-D10D-4F5D-9359-4CE46E4F96B9}")
	IIDICoreWebView2PrintSettings                                           = NewGUID("{377F3721-C74E-48CA-8DB1-DF68E51D60E2}")
	IIDICoreWebView2PrintSettings2                                          = NewGUID("{CA7F0E1F-3484-41D1-8C1A-65CD44A63F8D}")
	IIDICoreWebView2ProcessExtendedInfo                                     = NewGUID("{AF4C4C2E-45DB-11EE-BE56-0242AC120002}")
	IIDICoreWebView2ProcessFailedEventArgs2                                 = NewGUID("{4DAB9422-46FA-4C3E-A5D2-41D2071D3680}")
	IIDICoreWebView2ProcessInfo                                             = NewGUID("{84FA7612-3F3D-4FBF-889D-FAD000492D72}")
	IIDICoreWebView2Profile                                                 = NewGUID("{79110AD3-CD5D-4373-8BC3-C60658F17A5F}")
	IIDICoreWebView2Profile6                                                = NewGUID("{BD82FA6A-1D65-4C33-B2B4-0393020CC61B}")
	IIDICoreWebView2Profile8                                                = NewGUID("{FBF70C2F-EB1F-4383-85A0-163E92044011}")
	IIDICoreWebView2ScriptDialogOpeningEventArgs                            = NewGUID("{7390BB70-ABE0-4843-9529-F143B31B03D6}")
	IIDICoreWebView2ScriptException                                         = NewGUID("{054DAE00-84A3-49FF-BC17-4012A90BC9FD}")
	IIDICoreWebView2ServerCertificateErrorDetectedEventArgs                 = NewGUID("{012193ED-7C13-48FF-969D-A84C1F432A14}")
	IIDICoreWebView2SharedBuffer                                            = NewGUID("{B747A495-0C6F-449E-97B8-2F81E9D6AB43}")
	IIDICoreWebView2WebResourceRequest                                      = NewGUID("{97055CD4-512C-4264-8B5F-E3F446CEA6A5}")
	IIDICoreWebView2WebResourceRequestedEventArgs                           = NewGUID("{453E667F-12C7-49D4-BE6D-DDBE7956F57A}")
	IIDICoreWebView2WebResourceResponse                                     = NewGUID("{AAFCC94F-FA27-48FD-97DF-830EF75AAEC9}")
	IIDICoreWebView2WebResourceResponseView                                 = NewGUID("{79701053-7759-4162-8F7D-F1B3F084928D}")
	IIDICoreWebView2WindowFeatures                                          = NewGUID("{5EAF559F-B46E-4397-8860-E422F287FF1E}")
	IIDICoreWebView2ObjectCollection                                        = NewGUID("{5CFEC11C-25BD-4E8D-9E1A-7ACDAEEEC047}")
	IIDICoreWebView2Notification                                            = NewGUID("{B7434D98-6BC8-419D-9DA5-FB5A96D4DACD}")
	IIDICoreWebView2NotificationReceivedEventArgs                           = NewGUID("{1512DD5B-5514-4F85-886E-21C3A4C9CFE6}")
	IIDICoreWebView2SaveAsUIShowingEventArgs                                = NewGUID("{55902952-0E0D-5AAA-A7D0-E833CDB34F62}")
	IIDICoreWebView2AcceleratorKeyPressedEventArgs2                         = NewGUID("{03B2C8C8-7799-4E34-BD66-ED26AA85F2BF}")
	IIDICoreWebView2BrowserProcessExitedEventArgs                           = NewGUID("{1F00663F-AF8C-4782-9CDD-DD01C52E34CB}")
	IIDICoreWebView2CompositionController2                                  = NewGUID("{0B6A3D24-49CB-4806-BA20-B5E0734A7B26}")
	IIDICoreWebView2DragStartingEventHandler                                = NewGUID("{3B149321-83C3-5D1F-B03F-A42899BC1C15}")
	IIDICoreWebView2CompositionController5                                  = NewGUID("{8D0F82EB-7C33-5A4C-9108-84CA28CCC3B4}")
	IIDICoreWebView2ContentLoadingEventArgs                                 = NewGUID("{0C8A1275-9B6B-4901-87AD-70DF25BAFA6E}")
	IIDICoreWebView2ContextMenuItemCollection                               = NewGUID("{F562A2F5-C415-45CF-B909-D4B7C1E276D3}")
	IIDICoreWebView2Controller2                                             = NewGUID("{C979903E-D4CA-4228-92EB-47EE3FA96EAB}")
	IIDICoreWebView2Controller4                                             = NewGUID("{97D418D5-A426-4E49-A151-E1A10F327D9E}")
	IIDICoreWebView2ControllerOptions2                                      = NewGUID("{06C991D8-9E7E-11ED-A8FC-0242AC120002}")
	IIDICoreWebView2ControllerOptions3                                      = NewGUID("{B32B191A-8998-57CA-B7CB-E04617E4CE4A}")
	IIDICoreWebView2ControllerOptions4                                      = NewGUID("{21EB052F-AD39-555E-824A-C87B091D4D36}")
	IIDICoreWebView2CrashReport                                             = NewGUID("{74128FE8-5080-5F46-941D-FB747B0BCD62}")
	IIDICoreWebView2DOMContentLoadedEventArgs                               = NewGUID("{16B1E21A-C503-44F2-84C9-70ABA5031283}")
	IIDICoreWebView2DedicatedWorkerDedicatedWorkerCreatedEventHandler       = NewGUID("{A85B1B35-F6D7-5AE6-9FD7-57905DEFF79F}")
	IIDICoreWebView2DedicatedWorkerDestroyingEventHandler                   = NewGUID("{72473D5F-CBA2-57CA-A42E-03610A349FEF}")
	IIDICoreWebView2DedicatedWorkerWebMessageReceivedEventHandler           = NewGUID("{B366218B-0BB8-58A3-AC33-F40A2235366E}")
	IIDICoreWebView2DedicatedWorker                                         = NewGUID("{66833876-EDBA-5A60-8508-7DA64504A9D2}")
	IIDICoreWebView2DedicatedWorkerCreatedEventArgs                         = NewGUID("{910C52AF-8129-516D-AB9A-B524E377D8AC}")
	IIDICoreWebView2Deferral                                                = NewGUID("{C10E7F7B-B585-46F0-A623-8BEFBF3E4EE0}")
	IIDICoreWebView2DevToolsProtocolEventReceivedEventArgs                  = NewGUID("{653C2959-BB3A-4377-8632-B58ADA4E66C4}")
	IIDICoreWebView2DevToolsProtocolEventReceivedEventArgs2                 = NewGUID("{2DC4959D-1494-4393-95BA-BEA4CB9EBD1B}")
	IIDICoreWebView2DevToolsProtocolEventReceivedEventHandler               = NewGUID("{E2FDA4BE-5456-406C-A261-3D452138362C}")
	IIDICoreWebView2DevToolsProtocolEventReceiver                           = NewGUID("{B32CA51A-8371-45E9-9317-AF021D080367}")
	IIDICoreWebView2DragStartingEventArgs                                   = NewGUID("{8B8D9C7E-2F1A-4E6B-9D5A-3C8F7B9E1A2D}")
	IIDICoreWebView2Environment10                                           = NewGUID("{EE0EB9DF-6F12-46CE-B53F-3F47B9C928E0}")
	IIDICoreWebView2Environment11                                           = NewGUID("{F0913DC6-A0EC-42EF-9805-91DFF3A2966A}")
	IIDICoreWebView2Environment12                                           = NewGUID("{F503DB9B-739F-48DD-B151-FDFCF253F54E}")
	IIDICoreWebView2GetProcessExtendedInfosCompletedHandler                 = NewGUID("{F45E55AA-3BC2-11EE-BE56-0242AC120002}")
	IIDICoreWebView2ProcessExtendedInfoCollection                           = NewGUID("{32EFA696-407A-11EE-BE56-0242AC120002}")
	IIDICoreWebView2Environment13                                           = NewGUID("{AF641F58-72B2-11EE-B962-0242AC120002}")
	IIDICoreWebView2Environment14                                           = NewGUID("{A5E9FAD9-C875-59DA-9BD7-473AA5CA1CEF}")
	IIDICoreWebView2Environment15                                           = NewGUID("{2AC5EBFB-E654-5961-A667-7971885C7B27}")
	IIDICoreWebView2Environment2                                            = NewGUID("{41F3632B-5EF4-404F-AD82-2D606C5A9A21}")
	IIDICoreWebView2CreateCoreWebView2CompositionControllerCompletedHandler = NewGUID("{02FAB84B-1428-4FB7-AD45-1B2E64736184}")
	IIDICoreWebView2Environment3                                            = NewGUID("{80A22AE3-BE7C-4CE2-AFE1-5A50056CDEEB}")
	IIDICoreWebView2Environment4                                            = NewGUID("{20944379-6DCF-41D6-A0A0-ABC0FC50DE0D}")
	IIDICoreWebView2BrowserProcessExitedEventHandler                        = NewGUID("{FA504257-A216-4911-A860-FE8825712861}")
	IIDICoreWebView2Environment5                                            = NewGUID("{319E423D-E0D7-4B8D-9254-AE9475DE9B17}")
	IIDICoreWebView2Environment6                                            = NewGUID("{E59EE362-ACBD-4857-9A8E-D3644D9459A9}")
	IIDICoreWebView2Environment7                                            = NewGUID("{43C22296-3BBD-43A4-9C00-5C0DF6DD29A2}")
	IIDICoreWebView2ProcessInfosChangedEventHandler                         = NewGUID("{F4AF0C39-44B9-40E9-8B11-0484CFB9E0A1}")
	IIDICoreWebView2ProcessInfoCollection                                   = NewGUID("{402B99CD-A0CC-4FA5-B7A5-51D86A1D2339}")
	IIDICoreWebView2Environment8                                            = NewGUID("{D6EB91DD-C3D2-45E5-BD29-6DC2BC4DE9CF}")
	IIDICoreWebView2Environment9                                            = NewGUID("{F06F41BF-4B5A-49D8-B9F6-FA16CD29F274}")
	IIDICoreWebView2EnvironmentOptions2                                     = NewGUID("{FF85C98A-1BA7-4A6B-90C8-2B752C89E9E2}")
	IIDICoreWebView2EnvironmentOptions3                                     = NewGUID("{4A5C436E-A9E3-4A2E-89C3-910D3513F5CC}")
	IIDICoreWebView2EnvironmentOptions4                                     = NewGUID("{AC52D13F-0D38-475A-9DCA-876580D6793E}")
	IIDICoreWebView2EnvironmentOptions5                                     = NewGUID("{0AE35D64-C47F-4464-814E-259C345D1501}")
	IIDICoreWebView2EnvironmentOptions6                                     = NewGUID("{57D29CC3-C84F-42A0-B0E2-EFFBD5E179DE}")
	IIDICoreWebView2EnvironmentOptions8                                     = NewGUID("{7C7ECF51-E918-5CAF-853C-E9A2BCC27775}")
	IIDICoreWebView2EnvironmentOptions7                                     = NewGUID("{C48D539F-E39F-441C-AE68-1F66E570BDC5}")
	IIDICoreWebView2CreateCoreWebView2EnvironmentCompletedHandler           = NewGUID("{4E8A3389-C9D8-4BD2-B6B5-124FEE6CC14D}")
	IIDICoreWebView2File                                                    = NewGUID("{F2C19559-6BC1-4583-A757-90021BE9AFEC}")
	IIDICoreWebView2FileSystemHandle                                        = NewGUID("{C65100AC-0DE2-5551-A362-23D9BD1D0E1F}")
	IIDICoreWebView2FindStartCompletedHandler                               = NewGUID("{6A90ECAF-44B0-5BD9-8F07-1967E17BE9FB}")
	IIDICoreWebView2FindActiveMatchIndexChangedEventHandler                 = NewGUID("{0054F514-9A8E-5876-AED5-30B37F8C86A5}")
	IIDICoreWebView2FindMatchCountChangedEventHandler                       = NewGUID("{DA0D6827-4254-5B10-A6D9-412076AFC9F3}")
	IIDICoreWebView2Find                                                    = NewGUID("{A3EC0F5F-DDBC-54ED-8546-AF75A785B9A6}")
	IIDICoreWebView2FindOptions                                             = NewGUID("{E82E3B2B-A4AF-5BC6-94C6-18B44157A16C}")
	IIDICoreWebView2FramePermissionRequestedEventHandler                    = NewGUID("{845D0EDD-8BD8-429B-9915-4821789F23E9}")
	IIDICoreWebView2Frame3                                                  = NewGUID("{B50D82CC-CC28-481D-9614-CB048895E6A0}")
	IIDICoreWebView2Frame4                                                  = NewGUID("{188782DC-92AA-4732-AB3C-FCC59F6F68B9}")
	IIDICoreWebView2Frame5                                                  = NewGUID("{99D199C4-7305-11EE-B962-0242AC120002}")
	IIDICoreWebView2FrameScreenCaptureStartingEventHandler                  = NewGUID("{A6C1D8AD-BB80-59C5-895B-FBA1698B9309}")
	IIDICoreWebView2Frame6                                                  = NewGUID("{0DE611FD-31E9-5DDC-9D71-95EDA26EFF32}")
	IIDICoreWebView2FrameChildFrameCreatedEventHandler                      = NewGUID("{569E40E7-46B7-563D-83AE-1073155664D7}")
	IIDICoreWebView2Frame7                                                  = NewGUID("{3598CFA2-D85D-5A9F-9228-4DDE1F59EC64}")
	IIDICoreWebView2FrameDedicatedWorkerCreatedEventHandler                 = NewGUID("{3E28D307-9D58-5306-8097-A79301DE4F05}")
	IIDICoreWebView2Frame8                                                  = NewGUID("{9DEFF655-E3D5-5B8F-9107-52E66B3EC3FC}")
	IIDICoreWebView2FrameCreatedEventArgs                                   = NewGUID("{4D6E7B5E-9BAA-11EB-A8B3-0242AC130003}")
	IIDICoreWebView2FrameInfo                                               = NewGUID("{DA86B8A1-BDF3-4F11-9955-528CEFA59727}")
	IIDICoreWebView2FrameInfoCollection                                     = NewGUID("{8F834154-D38E-4D90-AFFB-6800A7272839}")
	IIDICoreWebView2FrameInfoCollectionIterator                             = NewGUID("{1BF89E2D-1B2B-4629-B28F-05099B41BB03}")
	IIDICoreWebView2HttpRequestHeaders                                      = NewGUID("{E86CAC0E-5523-465C-B536-8FB9FC8C8C60}")
	IIDICoreWebView2HttpResponseHeaders                                     = NewGUID("{03C5FF5A-9B45-4A88-881C-89A9F328619C}")
	IIDICoreWebView2NavigationCompletedEventArgs2                           = NewGUID("{FDF8B738-EE1E-4DB2-A329-8D7D7B74D792}")
	IIDICoreWebView2NavigationStartingEventArgs2                            = NewGUID("{9086BE93-91AA-472D-A7E0-579F2BA006AD}")
	IIDICoreWebView2NavigationStartingEventArgs3                            = NewGUID("{DDFFE494-4942-4BD2-AB73-35B8FF40E19F}")
	IIDICoreWebView2NewWindowRequestedEventArgs2                            = NewGUID("{BBC7BAED-74C6-4C92-B63A-7F5AEAE03DE3}")
	IIDICoreWebView2NewWindowRequestedEventArgs3                            = NewGUID("{842BED3C-6AD6-4DD9-B938-28C96667AD66}")
	IIDICoreWebView2NonClientRegionChangedEventArgs                         = NewGUID("{AB71D500-0820-4A52-809C-48DB04FF93BF}")
	IIDICoreWebView2ObjectCollectionView                                    = NewGUID("{0F36FD87-4F69-4415-98DA-888F89FB9A33}")
	IIDICoreWebView2PermissionRequestedEventArgs2                           = NewGUID("{74D7127F-9DE6-4200-8734-42D6FB4FF741}")
	IIDICoreWebView2PermissionRequestedEventArgs3                           = NewGUID("{E61670BC-3DCE-4177-86D2-C629AE3CB6AC}")
	IIDICoreWebView2PermissionSetting                                       = NewGUID("{792B6ECA-5576-421C-9119-74EBB3A4FFB3}")
	IIDICoreWebView2ProcessFailedEventArgs                                  = NewGUID("{8155A9A4-1474-4A86-8CAE-151B0FA6B8CA}")
	IIDICoreWebView2ProcessFailedEventArgs3                                 = NewGUID("{AB667428-094D-5FD1-B480-8B4C0FDBDF2F}")
	IIDICoreWebView2ProcessFailedEventArgs4                                 = NewGUID("{E00E3C7F-D6D3-5E64-B7AB-581B6316B227}")
	IIDICoreWebView2Profile9                                                = NewGUID("{40C764D0-12FC-5D23-816D-971B353174C1}")
	IIDICoreWebView2ClearBrowsingDataCompletedHandler                       = NewGUID("{E9710A06-1D1D-49B2-8234-226F35846AE5}")
	IIDICoreWebView2Profile2                                                = NewGUID("{FA740D4B-5EAE-4344-A8AD-74BE31925397}")
	IIDICoreWebView2Profile3                                                = NewGUID("{B188E659-5685-4E05-BDBA-FC640E0F1992}")
	IIDICoreWebView2SetPermissionStateCompletedHandler                      = NewGUID("{FC77FB30-9C9E-4076-B8C7-7644A703CA1B}")
	IIDICoreWebView2GetNonDefaultPermissionSettingsCompletedHandler         = NewGUID("{38274481-A15C-4563-94CF-990EDC9AEB95}")
	IIDICoreWebView2Profile4                                                = NewGUID("{8F4AE680-192E-4EC8-833A-21CFADAEF628}")
	IIDICoreWebView2Profile5                                                = NewGUID("{2EE5B76E-6E80-4DF2-BCD3-D4EC3340A01B}")
	IIDICoreWebView2ProfileAddBrowserExtensionCompletedHandler              = NewGUID("{DF1AAB27-82B9-4AB6-AAE8-017A49398C14}")
	IIDICoreWebView2ProfileGetBrowserExtensionsCompletedHandler             = NewGUID("{FCE16A1C-F107-4601-8B75-FC4940AE25D0}")
	IIDICoreWebView2BrowserExtensionList                                    = NewGUID("{2EF3D2DC-BD5F-4F4D-90AF-FD67798F0C2F}")
	IIDICoreWebView2Profile7                                                = NewGUID("{7B4C7906-A1AA-4CB4-B723-DB09F813D541}")
	IIDICoreWebView2SaveFileSecurityCheckStartingEventArgs                  = NewGUID("{CF4FF1D1-5A67-5660-8D63-EF699881EA65}")
	IIDICoreWebView2ScreenCaptureStartingEventArgs                          = NewGUID("{892C03FD-AEE3-5EBA-A1FA-6FD2F6484B2B}")
	IIDICoreWebView2ServiceWorkerDestroyingEventHandler                     = NewGUID("{C7528449-5674-5E34-B990-FF4CEF046214}")
	IIDICoreWebView2ServiceWorkerWebMessageReceivedEventHandler             = NewGUID("{65F0A1A6-A295-5A9F-8041-70DB71566F98}")
	IIDICoreWebView2ServiceWorker                                           = NewGUID("{245A7BB7-7C31-582D-95EE-0F2D99D2F5B0}")
	IIDICoreWebView2ServiceWorkerActivatedEventArgs                         = NewGUID("{5685C4B6-A514-58B2-9721-B61EF4CCD9D8}")
	IIDICoreWebView2GetServiceWorkerRegistrationsCompletedHandler           = NewGUID("{EA83432F-6528-592F-903B-0917EB0FD9C7}")
	IIDICoreWebView2ServiceWorkerRegisteredEventHandler                     = NewGUID("{C024C910-4DF2-54A3-B5C1-6152CA0E4577}")
	IIDICoreWebView2ServiceWorkerRegistrationCollectionView                 = NewGUID("{83D8CFA9-EF83-5447-9431-91C203C4C9D8}")
	IIDICoreWebView2ServiceWorkerManager                                    = NewGUID("{A2D52FD4-9B10-5971-8499-C67D1560F47A}")
	IIDICoreWebView2ServiceWorkerRegisteredEventArgs                        = NewGUID("{C6172932-407A-553B-B4D1-CDD12605EB6A}")
	IIDICoreWebView2ServiceWorkerActivatedEventHandler                      = NewGUID("{ECAA9897-769D-5ECE-A1E5-8859818CAF86}")
	IIDICoreWebView2ServiceWorkerRegistrationUnregisteringEventHandler      = NewGUID("{BEC01D14-6AD9-5257-9FF6-84F01BAA0CCA}")
	IIDICoreWebView2ServiceWorkerRegistration                               = NewGUID("{06003F5E-AF92-5E7E-B497-3FA167DD37C2}")
	IIDICoreWebView2Settings2                                               = NewGUID("{EE9A0F68-F46C-4E32-AC23-EF8CAC224D2A}")
	IIDICoreWebView2Settings3                                               = NewGUID("{FDB5AB74-AF33-4854-84F0-0A631DEB5EBA}")
	IIDICoreWebView2Settings5                                               = NewGUID("{183E7052-1D03-43A0-AB99-98E043B66B39}")
	IIDICoreWebView2Settings6                                               = NewGUID("{11CB3ACD-9BC8-43B8-83BF-F40753714F87}")
	IIDICoreWebView2Settings7                                               = NewGUID("{488DC902-35EF-42D2-BC7D-94B65C4BC49C}")
	IIDICoreWebView2Settings8                                               = NewGUID("{9E6B0E8F-86AD-4E81-8147-A9B5EDB68650}")
	IIDICoreWebView2Settings9                                               = NewGUID("{0528A73B-E92D-49F4-927A-E547DDDAA37D}")
	IIDICoreWebView2SharedWorkerDestroyingEventHandler                      = NewGUID("{5A459F84-FD04-5CCE-A998-6FAB56F09EEB}")
	IIDICoreWebView2SharedWorker                                            = NewGUID("{BCD39C8D-48BB-5F1B-BE22-89F9C0C4484A}")
	IIDICoreWebView2SharedWorkerCreatedEventArgs                            = NewGUID("{9F6615B0-08F1-5BAA-9C95-A02A1DC56D3F}")
	IIDICoreWebView2GetSharedWorkersCompletedHandler                        = NewGUID("{1F3179AE-15E5-51E4-8583-BE0CAF85ADC7}")
	IIDICoreWebView2SharedWorkerCreatedEventHandler                         = NewGUID("{79CB8524-B842-551A-8D31-5F824B6955ED}")
	IIDICoreWebView2SharedWorkerCollectionView                              = NewGUID("{F8842B09-0108-5575-A965-3D76FD267050}")
	IIDICoreWebView2SharedWorkerManager                                     = NewGUID("{9B897103-D035-551F-892E-3E8F2916D03E}")
	IIDICoreWebView2SourceChangedEventArgs                                  = NewGUID("{31E0E545-1DBA-4266-8914-F63848A1F7D7}")
	IIDICoreWebView2WebMessageReceivedEventArgs                             = NewGUID("{0F99A40C-E962-4207-9E92-E3D542EFF849}")
	IIDICoreWebView2WebMessageReceivedEventArgs2                            = NewGUID("{06FC7AB7-C90C-4297-9389-33CA01CF6D5E}")
	IIDICoreWebView2WebResourceRequestedEventArgs2                          = NewGUID("{9C562C24-B219-4D7F-92F6-B187FBBADD56}")
	IIDICoreWebView2WebResourceResponseReceivedEventArgs                    = NewGUID("{D1DB483D-6796-4B8B-80FC-13712BB716F4}")
	IIDICoreWebView2BasicAuthenticationRequestedEventHandler                = NewGUID("{58B4D6C2-18D4-497E-B39B-9A96533FA278}")
	IIDICoreWebView2_10                                                     = NewGUID("{B1690564-6F5A-4983-8E48-31D1143FECDB}")
	IIDICoreWebView2_13                                                     = NewGUID("{F75F09A8-667E-4983-88D6-C8773F315E84}")
	IIDICoreWebView2ClearServerCertificateErrorActionsCompletedHandler      = NewGUID("{3B40AAC6-ACFE-4FFD-8211-F607B96E2D5B}")
	IIDICoreWebView2ServerCertificateErrorDetectedEventHandler              = NewGUID("{969B3A26-D85E-4795-8199-FEF57344DA22}")
	IIDICoreWebView2_14                                                     = NewGUID("{6DAA4F10-4A90-4753-8898-77C5DF534165}")
	IIDICoreWebView2_17                                                     = NewGUID("{702E75D4-FD44-434D-9D70-1A68A6B1192A}")
	IIDICoreWebView2LaunchingExternalURISchemeEventHandler                  = NewGUID("{74F712E0-8165-43A9-A13F-0CCE597E75DF}")
	IIDICoreWebView2_18                                                     = NewGUID("{7A626017-28BE-49B2-B865-3BA2B3522D90}")
	IIDICoreWebView2_19                                                     = NewGUID("{6921F954-79B0-437F-A997-C85811897C68}")
	IIDICoreWebView2_20                                                     = NewGUID("{B4BC1926-7305-11EE-B962-0242AC120002}")
	IIDICoreWebView2ExecuteScriptWithResultCompletedHandler                 = NewGUID("{1BB5317B-8238-4C67-A7FF-BAF6558F289D}")
	IIDICoreWebView2_21                                                     = NewGUID("{C4980DEA-587B-43B9-8143-3EF3BF552D95}")
	IIDICoreWebView2_22                                                     = NewGUID("{DB75DFC7-A857-4632-A398-6969DDE26C0A}")
	IIDICoreWebView2ShowSaveAsUICompletedHandler                            = NewGUID("{E24B07E3-8169-5C34-994A-7F6478946A3C}")
	IIDICoreWebView2SaveAsUIShowingEventHandler                             = NewGUID("{6BAA177E-3A2E-5CCF-9A13-FAD676CD0522}")
	IIDICoreWebView2_25                                                     = NewGUID("{B5A86092-DF50-5B4F-A17B-6C8F8B40B771}")
	IIDICoreWebView2_23                                                     = NewGUID("{508F0DB5-90C4-5872-90A7-267A91377502}")
	IIDICoreWebView2ScreenCaptureStartingEventHandler                       = NewGUID("{E24FF05A-1DB5-59D9-89F3-3C864268DB4A}")
	IIDICoreWebView2_27                                                     = NewGUID("{00FBE33B-8C07-517C-AA23-0DDD4B5F6FA0}")
	IIDICoreWebView2_28                                                     = NewGUID("{62E50381-5BF5-51A8-AAE0-F20A3A9C8A90}")
	IIDICoreWebView2SaveFileSecurityCheckStartingEventHandler               = NewGUID("{7899576C-19E3-57C8-B7D1-55808292DE57}")
	IIDICoreWebView2_26                                                     = NewGUID("{806268B8-F897-5685-88E5-C45FCA0B1A48}")
	IIDICoreWebView2NotificationReceivedEventHandler                        = NewGUID("{89C5D598-8788-423B-BE97-E6E01C0F9EE3}")
	IIDICoreWebView2_24                                                     = NewGUID("{39A7AD55-4287-5CC1-88A1-C6F458593824}")
	IIDICoreWebView2DedicatedWorkerCreatedEventHandler                      = NewGUID("{CBA7462C-52C1-5706-8BBC-E9FC36476DE4}")
	IIDICoreWebView2_29                                                     = NewGUID("{650F917E-CC16-5C8C-9287-B0BBC8A4AB2C}")
	IIDICoreWebView2ClientCertificateRequestedEventHandler                  = NewGUID("{D7175BA2-BCC3-11EB-8529-0242AC130003}")
	IIDICoreWebView2_5                                                      = NewGUID("{BEDB11B8-D63C-11EB-B8BC-0242AC130003}")
	IIDICoreWebView2_6                                                      = NewGUID("{499AADAC-D92C-4589-8A75-111BFC167795}")
	IIDICoreWebView2PrintToPdfCompletedHandler                              = NewGUID("{CCF1EF04-FD8E-4D5F-B2DE-0983E41B8C36}")
	IIDICoreWebView2_7                                                      = NewGUID("{79C24D83-09A3-45AE-9418-487F32A58740}")
)

type ICoreWebView2ExecuteScriptCompletedHandlerVtbl struct {
	IUnknownVtbl

	Invoke ComProc
}

type ICoreWebView2ExecuteScriptCompletedHandler struct {
	//lint:ignore U1000 campo lido pelo Windows via COM, Go nunca le
	vtbl *ICoreWebView2ExecuteScriptCompletedHandlerVtbl
}

type ICoreWebView2CallDevToolsProtocolMethodCompletedHandlerVtbl struct {
	IUnknownVtbl

	Invoke ComProc
}

type ICoreWebView2CallDevToolsProtocolMethodCompletedHandler struct {
	//lint:ignore U1000 campo lido pelo Windows via COM, Go nunca le
	vtbl *ICoreWebView2CallDevToolsProtocolMethodCompletedHandlerVtbl
}

type ICoreWebView2DocumentTitleChangedEventHandlerVtbl struct {
	IUnknownVtbl

	Invoke ComProc
}

type ICoreWebView2DocumentTitleChangedEventHandler struct {
	//lint:ignore U1000 campo lido pelo Windows via COM, Go nunca le
	vtbl *ICoreWebView2DocumentTitleChangedEventHandlerVtbl
}

type ICoreWebView2NavigationCompletedEventHandlerVtbl struct {
	IUnknownVtbl

	Invoke ComProc
}

type ICoreWebView2NavigationCompletedEventHandler struct {
	//lint:ignore U1000 campo lido pelo Windows via COM, Go nunca le
	vtbl *ICoreWebView2NavigationCompletedEventHandlerVtbl
}

type ICoreWebView2NavigationStartingEventHandlerVtbl struct {
	IUnknownVtbl

	Invoke ComProc
}

type ICoreWebView2NavigationStartingEventHandler struct {
	//lint:ignore U1000 campo lido pelo Windows via COM, Go nunca le
	vtbl *ICoreWebView2NavigationStartingEventHandlerVtbl
}

type ICoreWebView2NewWindowRequestedEventHandlerVtbl struct {
	IUnknownVtbl

	Invoke ComProc
}

type ICoreWebView2NewWindowRequestedEventHandler struct {
	//lint:ignore U1000 campo lido pelo Windows via COM, Go nunca le
	vtbl *ICoreWebView2NewWindowRequestedEventHandlerVtbl
}

type ICoreWebView2PermissionRequestedEventHandlerVtbl struct {
	IUnknownVtbl

	Invoke ComProc
}

type ICoreWebView2PermissionRequestedEventHandler struct {
	//lint:ignore U1000 campo lido pelo Windows via COM, Go nunca le
	vtbl *ICoreWebView2PermissionRequestedEventHandlerVtbl
}

type ICoreWebView2ProcessFailedEventHandlerVtbl struct {
	IUnknownVtbl

	Invoke ComProc
}

type ICoreWebView2ProcessFailedEventHandler struct {
	//lint:ignore U1000 campo lido pelo Windows via COM, Go nunca le
	vtbl *ICoreWebView2ProcessFailedEventHandlerVtbl
}

type ICoreWebView2SourceChangedEventHandlerVtbl struct {
	IUnknownVtbl

	Invoke ComProc
}

type ICoreWebView2SourceChangedEventHandler struct {
	//lint:ignore U1000 campo lido pelo Windows via COM, Go nunca le
	vtbl *ICoreWebView2SourceChangedEventHandlerVtbl
}

type ICoreWebView2WebMessageReceivedEventHandlerVtbl struct {
	IUnknownVtbl

	Invoke ComProc
}

type ICoreWebView2WebMessageReceivedEventHandler struct {
	//lint:ignore U1000 campo lido pelo Windows via COM, Go nunca le
	vtbl *ICoreWebView2WebMessageReceivedEventHandlerVtbl
}

type ICoreWebView2WebResourceRequestedEventHandlerVtbl struct {
	IUnknownVtbl

	Invoke ComProc
}

type ICoreWebView2WebResourceRequestedEventHandler struct {
	//lint:ignore U1000 campo lido pelo Windows via COM, Go nunca le
	vtbl *ICoreWebView2WebResourceRequestedEventHandlerVtbl
}

type ICoreWebView2TrySuspendCompletedHandlerVtbl struct {
	IUnknownVtbl

	Invoke ComProc
}

type ICoreWebView2TrySuspendCompletedHandler struct {
	//lint:ignore U1000 campo lido pelo Windows via COM, Go nunca le
	vtbl *ICoreWebView2TrySuspendCompletedHandlerVtbl
}

type ICoreWebView2ShowSaveAsUICompletedHandlerVtbl struct {
	IUnknownVtbl

	Invoke ComProc
}

type ICoreWebView2ShowSaveAsUICompletedHandler struct {
	//lint:ignore U1000 campo lido pelo Windows via COM, Go nunca le
	vtbl *ICoreWebView2ShowSaveAsUICompletedHandlerVtbl
}

type ICoreWebView2DownloadStartingEventHandlerVtbl struct {
	IUnknownVtbl

	Invoke ComProc
}

type ICoreWebView2DownloadStartingEventHandler struct {
	//lint:ignore U1000 campo lido pelo Windows via COM, Go nunca le
	vtbl *ICoreWebView2DownloadStartingEventHandlerVtbl
}

type ICoreWebView2IsDocumentPlayingAudioChangedEventHandlerVtbl struct {
	IUnknownVtbl

	Invoke ComProc
}

type ICoreWebView2IsDocumentPlayingAudioChangedEventHandler struct {
	//lint:ignore U1000 campo lido pelo Windows via COM, Go nunca le
	vtbl *ICoreWebView2IsDocumentPlayingAudioChangedEventHandlerVtbl
}

type ICoreWebView2IsMutedChangedEventHandlerVtbl struct {
	IUnknownVtbl

	Invoke ComProc
}

type ICoreWebView2IsMutedChangedEventHandler struct {
	//lint:ignore U1000 campo lido pelo Windows via COM, Go nunca le
	vtbl *ICoreWebView2IsMutedChangedEventHandlerVtbl
}

type ICoreWebView2ContextMenuRequestedEventHandlerVtbl struct {
	IUnknownVtbl

	Invoke ComProc
}

type ICoreWebView2ContextMenuRequestedEventHandler struct {
	//lint:ignore U1000 campo lido pelo Windows via COM, Go nunca le
	vtbl *ICoreWebView2ContextMenuRequestedEventHandlerVtbl
}

type ICoreWebView2AcceleratorKeyPressedEventHandlerVtbl struct {
	IUnknownVtbl

	Invoke ComProc
}

type ICoreWebView2AcceleratorKeyPressedEventHandler struct {
	//lint:ignore U1000 campo lido pelo Windows via COM, Go nunca le
	vtbl *ICoreWebView2AcceleratorKeyPressedEventHandlerVtbl
}

type ICoreWebView2FocusChangedEventHandlerVtbl struct {
	IUnknownVtbl

	Invoke ComProc
}

type ICoreWebView2FocusChangedEventHandler struct {
	//lint:ignore U1000 campo lido pelo Windows via COM, Go nunca le
	vtbl *ICoreWebView2FocusChangedEventHandlerVtbl
}

type ICoreWebView2CreateCoreWebView2ControllerCompletedHandlerVtbl struct {
	IUnknownVtbl

	Invoke ComProc
}

type ICoreWebView2CreateCoreWebView2ControllerCompletedHandler struct {
	//lint:ignore U1000 campo lido pelo Windows via COM, Go nunca le
	vtbl *ICoreWebView2CreateCoreWebView2ControllerCompletedHandlerVtbl
}

type ICoreWebView2BrowserExtensionRemoveCompletedHandlerVtbl struct {
	IUnknownVtbl

	Invoke ComProc
}

type ICoreWebView2BrowserExtensionRemoveCompletedHandler struct {
	//lint:ignore U1000 campo lido pelo Windows via COM, Go nunca le
	vtbl *ICoreWebView2BrowserExtensionRemoveCompletedHandlerVtbl
}

type ICoreWebView2BrowserExtensionEnableCompletedHandlerVtbl struct {
	IUnknownVtbl

	Invoke ComProc
}

type ICoreWebView2BrowserExtensionEnableCompletedHandler struct {
	//lint:ignore U1000 campo lido pelo Windows via COM, Go nunca le
	vtbl *ICoreWebView2BrowserExtensionEnableCompletedHandlerVtbl
}

type ICoreWebView2CustomItemSelectedEventHandlerVtbl struct {
	IUnknownVtbl

	Invoke ComProc
}

type ICoreWebView2CustomItemSelectedEventHandler struct {
	//lint:ignore U1000 campo lido pelo Windows via COM, Go nunca le
	vtbl *ICoreWebView2CustomItemSelectedEventHandlerVtbl
}

type ICoreWebView2BytesReceivedChangedEventHandlerVtbl struct {
	IUnknownVtbl

	Invoke ComProc
}

type ICoreWebView2BytesReceivedChangedEventHandler struct {
	//lint:ignore U1000 campo lido pelo Windows via COM, Go nunca le
	vtbl *ICoreWebView2BytesReceivedChangedEventHandlerVtbl
}

type ICoreWebView2StateChangedEventHandlerVtbl struct {
	IUnknownVtbl

	Invoke ComProc
}

type ICoreWebView2StateChangedEventHandler struct {
	//lint:ignore U1000 campo lido pelo Windows via COM, Go nunca le
	vtbl *ICoreWebView2StateChangedEventHandlerVtbl
}

type ICoreWebView2Vtbl struct {
	IUnknownVtbl

	GetSettings                            ComProc
	GetSource                              ComProc
	Navigate                               ComProc
	NavigateToString                       ComProc
	AddNavigationStarting                  ComProc
	RemoveNavigationStarting               ComProc
	AddContentLoading                      ComProc
	RemoveContentLoading                   ComProc
	AddSourceChanged                       ComProc
	RemoveSourceChanged                    ComProc
	AddHistoryChanged                      ComProc
	RemoveHistoryChanged                   ComProc
	AddNavigationCompleted                 ComProc
	RemoveNavigationCompleted              ComProc
	AddFrameNavigationStarting             ComProc
	RemoveFrameNavigationStarting          ComProc
	AddFrameNavigationCompleted            ComProc
	RemoveFrameNavigationCompleted         ComProc
	AddScriptDialogOpening                 ComProc
	RemoveScriptDialogOpening              ComProc
	AddPermissionRequested                 ComProc
	RemovePermissionRequested              ComProc
	AddProcessFailed                       ComProc
	RemoveProcessFailed                    ComProc
	AddScriptToExecuteOnDocumentCreated    ComProc
	RemoveScriptToExecuteOnDocumentCreated ComProc
	ExecuteScript                          ComProc
	CapturePreview                         ComProc
	Reload                                 ComProc
	PostWebMessageAsJSON                   ComProc
	PostWebMessageAsString                 ComProc
	AddWebMessageReceived                  ComProc
	RemoveWebMessageReceived               ComProc
	CallDevToolsProtocolMethod             ComProc
	GetBrowserProcessID                    ComProc
	GetCanGoBack                           ComProc
	GetCanGoForward                        ComProc
	GoBack                                 ComProc
	GoForward                              ComProc
	GetDevToolsProtocolEventReceiver       ComProc
	Stop                                   ComProc
	AddNewWindowRequested                  ComProc
	RemoveNewWindowRequested               ComProc
	AddDocumentTitleChanged                ComProc
	RemoveDocumentTitleChanged             ComProc
	GetDocumentTitle                       ComProc
	AddHostObjectToScript                  ComProc
	RemoveHostObjectFromScript             ComProc
	OpenDevToolsWindow                     ComProc
	AddContainsFullScreenElementChanged    ComProc
	RemoveContainsFullScreenElementChanged ComProc
	GetContainsFullScreenElement           ComProc
	AddWebResourceRequested                ComProc
	RemoveWebResourceRequested             ComProc
	AddWebResourceRequestedFilter          ComProc
	RemoveWebResourceRequestedFilter       ComProc
	AddWindowCloseRequested                ComProc
	RemoveWindowCloseRequested             ComProc
}

type ICoreWebView2 struct {
	vtbl *ICoreWebView2Vtbl
}

type ICoreWebView2_2Vtbl struct {
	ICoreWebView2Vtbl

	AddWebResourceResponseReceived    ComProc
	RemoveWebResourceResponseReceived ComProc
	NavigateWithWebResourceRequest    ComProc
	AddDOMContentLoaded               ComProc
	RemoveDOMContentLoaded            ComProc
	GetCookieManager                  ComProc
	GetEnvironment                    ComProc
}

type ICoreWebView2_2 struct {
	//lint:ignore U1000 campo lido pelo Windows via COM, Go nunca le
	vtbl *ICoreWebView2_2Vtbl
}

type ICoreWebView2_3Vtbl struct {
	ICoreWebView2_2Vtbl

	TrySuspend                          ComProc
	Resume                              ComProc
	GetIsSuspended                      ComProc
	SetVirtualHostNameToFolderMapping   ComProc
	ClearVirtualHostNameToFolderMapping ComProc
}

type ICoreWebView2_3 struct {
	vtbl *ICoreWebView2_3Vtbl
}

type ICoreWebView2_4Vtbl struct {
	ICoreWebView2_3Vtbl

	AddFrameCreated        ComProc
	RemoveFrameCreated     ComProc
	AddDownloadStarting    ComProc
	RemoveDownloadStarting ComProc
}

type ICoreWebView2_4 struct {
	vtbl *ICoreWebView2_4Vtbl
}

type ICoreWebView2_8Vtbl struct {
	ICoreWebView2_7Vtbl

	AddIsMutedChanged                   ComProc
	RemoveIsMutedChanged                ComProc
	GetIsMuted                          ComProc
	PutIsMuted                          ComProc
	AddIsDocumentPlayingAudioChanged    ComProc
	RemoveIsDocumentPlayingAudioChanged ComProc
	GetIsDocumentPlayingAudio           ComProc
}

type ICoreWebView2_8 struct {
	vtbl *ICoreWebView2_8Vtbl
}

type ICoreWebView2_9Vtbl struct {
	ICoreWebView2_8Vtbl

	AddIsDefaultDownloadDialogOpenChanged    ComProc
	RemoveIsDefaultDownloadDialogOpenChanged ComProc
	GetIsDefaultDownloadDialogOpen           ComProc
	OpenDefaultDownloadDialog                ComProc
	CloseDefaultDownloadDialog               ComProc
	GetDefaultDownloadDialogCornerAlignment  ComProc
	PutDefaultDownloadDialogCornerAlignment  ComProc
	GetDefaultDownloadDialogMargin           ComProc
	PutDefaultDownloadDialogMargin           ComProc
}

type ICoreWebView2_9 struct {
	//lint:ignore U1000 campo lido pelo Windows via COM, Go nunca le
	vtbl *ICoreWebView2_9Vtbl
}

type ICoreWebView2_11Vtbl struct {
	ICoreWebView2_10Vtbl

	CallDevToolsProtocolMethodForSession ComProc
	AddContextMenuRequested              ComProc
	RemoveContextMenuRequested           ComProc
}

type ICoreWebView2_11 struct {
	vtbl *ICoreWebView2_11Vtbl
}

type ICoreWebView2_12Vtbl struct {
	ICoreWebView2_11Vtbl

	AddStatusBarTextChanged    ComProc
	RemoveStatusBarTextChanged ComProc
	GetStatusBarText           ComProc
}

type ICoreWebView2_12 struct {
	//lint:ignore U1000 campo lido pelo Windows via COM, Go nunca le
	vtbl *ICoreWebView2_12Vtbl
}

type ICoreWebView2_15Vtbl struct {
	ICoreWebView2_14Vtbl

	AddFaviconChanged    ComProc
	RemoveFaviconChanged ComProc
	GetFaviconURI        ComProc
	GetFavicon           ComProc
}

type ICoreWebView2_15 struct {
	//lint:ignore U1000 campo lido pelo Windows via COM, Go nunca le
	vtbl *ICoreWebView2_15Vtbl
}

type ICoreWebView2_16Vtbl struct {
	ICoreWebView2_15Vtbl

	Print            ComProc
	ShowPrintUI      ComProc
	PrintToPdfStream ComProc
}

type ICoreWebView2_16 struct {
	//lint:ignore U1000 campo lido pelo Windows via COM, Go nunca le
	vtbl *ICoreWebView2_16Vtbl
}

type ICoreWebView2SettingsVtbl struct {
	IUnknownVtbl

	GetIsScriptEnabled                ComProc
	PutIsScriptEnabled                ComProc
	GetIsWebMessageEnabled            ComProc
	PutIsWebMessageEnabled            ComProc
	GetAreDefaultScriptDialogsEnabled ComProc
	PutAreDefaultScriptDialogsEnabled ComProc
	GetIsStatusBarEnabled             ComProc
	PutIsStatusBarEnabled             ComProc
	GetAreDevToolsEnabled             ComProc
	PutAreDevToolsEnabled             ComProc
	GetAreDefaultContextMenusEnabled  ComProc
	PutAreDefaultContextMenusEnabled  ComProc
	GetAreHostObjectsAllowed          ComProc
	PutAreHostObjectsAllowed          ComProc
	GetIsZoomControlEnabled           ComProc
	PutIsZoomControlEnabled           ComProc
	GetIsBuiltInErrorPageEnabled      ComProc
	PutIsBuiltInErrorPageEnabled      ComProc
}

type ICoreWebView2Settings struct {
	vtbl *ICoreWebView2SettingsVtbl
}

type ICoreWebView2Settings4Vtbl struct {
	ICoreWebView2Settings3Vtbl

	GetIsPasswordAutosaveEnabled ComProc
	PutIsPasswordAutosaveEnabled ComProc
	GetIsGeneralAutofillEnabled  ComProc
	PutIsGeneralAutofillEnabled  ComProc
}

type ICoreWebView2Settings4 struct {
	//lint:ignore U1000 campo lido pelo Windows via COM, Go nunca le
	vtbl *ICoreWebView2Settings4Vtbl
}

type ICoreWebView2ControllerVtbl struct {
	IUnknownVtbl

	GetIsVisible                      ComProc
	PutIsVisible                      ComProc
	GetBounds                         ComProc
	PutBounds                         ComProc
	GetZoomFactor                     ComProc
	PutZoomFactor                     ComProc
	AddZoomFactorChanged              ComProc
	RemoveZoomFactorChanged           ComProc
	SetBoundsAndZoomFactor            ComProc
	MoveFocus                         ComProc
	AddMoveFocusRequested             ComProc
	RemoveMoveFocusRequested          ComProc
	AddGotFocus                       ComProc
	RemoveGotFocus                    ComProc
	AddLostFocus                      ComProc
	RemoveLostFocus                   ComProc
	AddAcceleratorKeyPressed          ComProc
	RemoveAcceleratorKeyPressed       ComProc
	GetParentWindow                   ComProc
	PutParentWindow                   ComProc
	NotifyParentWindowPositionChanged ComProc
	Close                             ComProc
	GetCoreWebView2                   ComProc
}

type ICoreWebView2Controller struct {
	vtbl *ICoreWebView2ControllerVtbl
}

type ICoreWebView2Controller3Vtbl struct {
	ICoreWebView2Controller2Vtbl

	GetRasterizationScale              ComProc
	PutRasterizationScale              ComProc
	GetShouldDetectMonitorScaleChanges ComProc
	PutShouldDetectMonitorScaleChanges ComProc
	AddRasterizationScaleChanged       ComProc
	RemoveRasterizationScaleChanged    ComProc
	GetBoundsMode                      ComProc
	PutBoundsMode                      ComProc
}

type ICoreWebView2Controller3 struct {
	vtbl *ICoreWebView2Controller3Vtbl
}

type ICoreWebView2EnvironmentVtbl struct {
	IUnknownVtbl

	CreateCoreWebView2Controller     ComProc
	CreateWebResourceResponse        ComProc
	GetBrowserVersionString          ComProc
	AddNewBrowserVersionAvailable    ComProc
	RemoveNewBrowserVersionAvailable ComProc
}

type ICoreWebView2Environment struct {
	vtbl *ICoreWebView2EnvironmentVtbl
}

type ICoreWebView2EnvironmentOptionsVtbl struct {
	IUnknownVtbl

	GetAdditionalBrowserArguments             ComProc
	PutAdditionalBrowserArguments             ComProc
	GetLanguage                               ComProc
	PutLanguage                               ComProc
	GetTargetCompatibleBrowserVersion         ComProc
	PutTargetCompatibleBrowserVersion         ComProc
	GetAllowSingleSignOnUsingOSPrimaryAccount ComProc
	PutAllowSingleSignOnUsingOSPrimaryAccount ComProc
}

type ICoreWebView2EnvironmentOptions struct {
	//lint:ignore U1000 campo lido pelo Windows via COM, Go nunca le
	vtbl *ICoreWebView2EnvironmentOptionsVtbl
}

type ICoreWebView2AcceleratorKeyPressedEventArgsVtbl struct {
	IUnknownVtbl

	GetKeyEventKind      ComProc
	GetVirtualKey        ComProc
	GetKeyEventLParam    ComProc
	GetPhysicalKeyStatus ComProc
	GetHandled           ComProc
	PutHandled           ComProc
}

type ICoreWebView2AcceleratorKeyPressedEventArgs struct {
	vtbl *ICoreWebView2AcceleratorKeyPressedEventArgsVtbl
}

type ICoreWebView2BrowserExtensionVtbl struct {
	IUnknownVtbl

	GetID        ComProc
	GetName      ComProc
	Remove       ComProc
	GetIsEnabled ComProc
	Enable       ComProc
}

type ICoreWebView2BrowserExtension struct {
	vtbl *ICoreWebView2BrowserExtensionVtbl
}

type ICoreWebView2CompositionControllerVtbl struct {
	IUnknownVtbl

	GetRootVisualTarget ComProc
	PutRootVisualTarget ComProc
	SendMouseInput      ComProc
	SendPointerInput    ComProc
	GetCursor           ComProc
	GetSystemCursorID   ComProc
	AddCursorChanged    ComProc
	RemoveCursorChanged ComProc
}

type ICoreWebView2CompositionController struct {
	//lint:ignore U1000 campo lido pelo Windows via COM, Go nunca le
	vtbl *ICoreWebView2CompositionControllerVtbl
}

type ICoreWebView2CompositionController3Vtbl struct {
	ICoreWebView2CompositionController2Vtbl

	DragEnter ComProc
	DragLeave ComProc
	DragOver  ComProc
	Drop      ComProc
}

type ICoreWebView2CompositionController3 struct {
	//lint:ignore U1000 campo lido pelo Windows via COM, Go nunca le
	vtbl *ICoreWebView2CompositionController3Vtbl
}

type ICoreWebView2CompositionController4Vtbl struct {
	ICoreWebView2CompositionController3Vtbl

	GetNonClientRegionAtPoint    ComProc
	QueryNonClientRegion         ComProc
	AddNonClientRegionChanged    ComProc
	RemoveNonClientRegionChanged ComProc
}

type ICoreWebView2CompositionController4 struct {
	//lint:ignore U1000 campo lido pelo Windows via COM, Go nunca le
	vtbl *ICoreWebView2CompositionController4Vtbl
}

type ICoreWebView2ContextMenuItemVtbl struct {
	IUnknownVtbl

	GetName                   ComProc
	GetLabel                  ComProc
	GetCommandID              ComProc
	GetShortcutKeyDescription ComProc
	GetIcon                   ComProc
	GetKind                   ComProc
	PutIsEnabled              ComProc
	GetIsEnabled              ComProc
	PutIsChecked              ComProc
	GetIsChecked              ComProc
	GetChildren               ComProc
	AddCustomItemSelected     ComProc
	RemoveCustomItemSelected  ComProc
}

type ICoreWebView2ContextMenuItem struct {
	vtbl *ICoreWebView2ContextMenuItemVtbl
}

type ICoreWebView2ContextMenuTargetVtbl struct {
	IUnknownVtbl

	GetKind                    ComProc
	GetIsEditable              ComProc
	GetIsRequestedForMainFrame ComProc
	GetPageURI                 ComProc
	GetFrameURI                ComProc
	GetHasLinkURI              ComProc
	GetLinkURI                 ComProc
	GetHasLinkText             ComProc
	GetLinkText                ComProc
	GetHasSourceURI            ComProc
	GetSourceURI               ComProc
	GetHasSelection            ComProc
	GetSelectionText           ComProc
}

type ICoreWebView2ContextMenuTarget struct {
	vtbl *ICoreWebView2ContextMenuTargetVtbl
}

type ICoreWebView2ContextMenuRequestedEventArgsVtbl struct {
	IUnknownVtbl

	GetMenuItems         ComProc
	GetContextMenuTarget ComProc
	GetLocation          ComProc
	PutSelectedCommandID ComProc
	GetSelectedCommandID ComProc
	PutHandled           ComProc
	GetHandled           ComProc
	GetDeferral          ComProc
}

type ICoreWebView2ContextMenuRequestedEventArgs struct {
	vtbl *ICoreWebView2ContextMenuRequestedEventArgsVtbl
}

type ICoreWebView2ControllerOptionsVtbl struct {
	IUnknownVtbl

	GetProfileName            ComProc
	PutProfileName            ComProc
	GetIsInPrivateModeEnabled ComProc
	PutIsInPrivateModeEnabled ComProc
}

type ICoreWebView2ControllerOptions struct {
	//lint:ignore U1000 campo lido pelo Windows via COM, Go nunca le
	vtbl *ICoreWebView2ControllerOptionsVtbl
}

type ICoreWebView2CustomSchemeRegistrationVtbl struct {
	IUnknownVtbl

	GetSchemeName            ComProc
	GetTreatAsSecure         ComProc
	PutTreatAsSecure         ComProc
	GetAllowedOrigins        ComProc
	SetAllowedOrigins        ComProc
	GetHasAuthorityComponent ComProc
	PutHasAuthorityComponent ComProc
}

type ICoreWebView2CustomSchemeRegistration struct {
	//lint:ignore U1000 campo lido pelo Windows via COM, Go nunca le
	vtbl *ICoreWebView2CustomSchemeRegistrationVtbl
}

type ICoreWebView2DownloadOperationVtbl struct {
	IUnknownVtbl

	AddBytesReceivedChanged       ComProc
	RemoveBytesReceivedChanged    ComProc
	AddEstimatedEndTimeChanged    ComProc
	RemoveEstimatedEndTimeChanged ComProc
	AddStateChanged               ComProc
	RemoveStateChanged            ComProc
	GetURI                        ComProc
	GetContentDisposition         ComProc
	GetMimeType                   ComProc
	GetTotalBytesToReceive        ComProc
	GetBytesReceived              ComProc
	GetEstimatedEndTime           ComProc
	GetResultFilePath             ComProc
	GetState                      ComProc
	GetInterruptReason            ComProc
	Cancel                        ComProc
	Pause                         ComProc
	Resume                        ComProc
	GetCanResume                  ComProc
}

type ICoreWebView2DownloadOperation struct {
	vtbl *ICoreWebView2DownloadOperationVtbl
}

type ICoreWebView2DownloadStartingEventArgsVtbl struct {
	IUnknownVtbl

	GetDownloadOperation ComProc
	GetCancel            ComProc
	PutCancel            ComProc
	GetResultFilePath    ComProc
	PutResultFilePath    ComProc
	GetHandled           ComProc
	PutHandled           ComProc
	GetDeferral          ComProc
}

type ICoreWebView2DownloadStartingEventArgs struct {
	vtbl *ICoreWebView2DownloadStartingEventArgsVtbl
}

type ICoreWebView2FrameVtbl struct {
	IUnknownVtbl

	GetName                          ComProc
	AddNameChanged                   ComProc
	RemoveNameChanged                ComProc
	AddHostObjectToScriptWithOrigins ComProc
	RemoveHostObjectFromScript       ComProc
	AddDestroyed                     ComProc
	RemoveDestroyed                  ComProc
	IsDestroyed                      ComProc
}

type ICoreWebView2Frame struct {
	//lint:ignore U1000 campo lido pelo Windows via COM, Go nunca le
	vtbl *ICoreWebView2FrameVtbl
}

type ICoreWebView2Frame2Vtbl struct {
	ICoreWebView2FrameVtbl

	AddNavigationStarting     ComProc
	RemoveNavigationStarting  ComProc
	AddContentLoading         ComProc
	RemoveContentLoading      ComProc
	AddNavigationCompleted    ComProc
	RemoveNavigationCompleted ComProc
	AddDOMContentLoaded       ComProc
	RemoveDOMContentLoaded    ComProc
	ExecuteScript             ComProc
	PostWebMessageAsJSON      ComProc
	PostWebMessageAsString    ComProc
	AddWebMessageReceived     ComProc
	RemoveWebMessageReceived  ComProc
}

type ICoreWebView2Frame2 struct {
	//lint:ignore U1000 campo lido pelo Windows via COM, Go nunca le
	vtbl *ICoreWebView2Frame2Vtbl
}

type ICoreWebView2NavigationCompletedEventArgsVtbl struct {
	IUnknownVtbl

	GetIsSuccess      ComProc
	GetWebErrorStatus ComProc
	GetNavigationID   ComProc
}

type ICoreWebView2NavigationCompletedEventArgs struct {
	vtbl *ICoreWebView2NavigationCompletedEventArgsVtbl
}

type ICoreWebView2NavigationStartingEventArgsVtbl struct {
	IUnknownVtbl

	GetURI             ComProc
	GetIsUserInitiated ComProc
	GetIsRedirected    ComProc
	GetRequestHeaders  ComProc
	GetCancel          ComProc
	PutCancel          ComProc
	GetNavigationID    ComProc
}

type ICoreWebView2NavigationStartingEventArgs struct {
	vtbl *ICoreWebView2NavigationStartingEventArgsVtbl
}

type ICoreWebView2NewWindowRequestedEventArgsVtbl struct {
	IUnknownVtbl

	GetURI             ComProc
	PutNewWindow       ComProc
	GetNewWindow       ComProc
	PutHandled         ComProc
	GetHandled         ComProc
	GetIsUserInitiated ComProc
	GetDeferral        ComProc
	GetWindowFeatures  ComProc
}

type ICoreWebView2NewWindowRequestedEventArgs struct {
	vtbl *ICoreWebView2NewWindowRequestedEventArgsVtbl
}

type ICoreWebView2PermissionRequestedEventArgsVtbl struct {
	IUnknownVtbl

	GetURI             ComProc
	GetPermissionKind  ComProc
	GetIsUserInitiated ComProc
	GetState           ComProc
	PutState           ComProc
	GetDeferral        ComProc
}

type ICoreWebView2PermissionRequestedEventArgs struct {
	vtbl *ICoreWebView2PermissionRequestedEventArgsVtbl
}

type ICoreWebView2PrintSettingsVtbl struct {
	IUnknownVtbl

	GetOrientation                ComProc
	PutOrientation                ComProc
	GetScaleFactor                ComProc
	PutScaleFactor                ComProc
	GetPageWidth                  ComProc
	PutPageWidth                  ComProc
	GetPageHeight                 ComProc
	PutPageHeight                 ComProc
	GetMarginTop                  ComProc
	PutMarginTop                  ComProc
	GetMarginBottom               ComProc
	PutMarginBottom               ComProc
	GetMarginLeft                 ComProc
	PutMarginLeft                 ComProc
	GetMarginRight                ComProc
	PutMarginRight                ComProc
	GetShouldPrintBackgrounds     ComProc
	PutShouldPrintBackgrounds     ComProc
	GetShouldPrintSelectionOnly   ComProc
	PutShouldPrintSelectionOnly   ComProc
	GetShouldPrintHeaderAndFooter ComProc
	PutShouldPrintHeaderAndFooter ComProc
	GetHeaderTitle                ComProc
	PutHeaderTitle                ComProc
	GetFooterURI                  ComProc
	PutFooterURI                  ComProc
}

type ICoreWebView2PrintSettings struct {
	//lint:ignore U1000 campo lido pelo Windows via COM, Go nunca le
	vtbl *ICoreWebView2PrintSettingsVtbl
}

type ICoreWebView2ProcessFailedEventArgs2Vtbl struct {
	ICoreWebView2ProcessFailedEventArgsVtbl

	GetReason                     ComProc
	GetExitCode                   ComProc
	GetProcessDescription         ComProc
	GetFrameInfosForFailedProcess ComProc
}

type ICoreWebView2ProcessFailedEventArgs2 struct {
	//lint:ignore U1000 campo lido pelo Windows via COM, Go nunca le
	vtbl *ICoreWebView2ProcessFailedEventArgs2Vtbl
}

type ICoreWebView2ProfileVtbl struct {
	IUnknownVtbl

	GetProfileName               ComProc
	GetIsInPrivateModeEnabled    ComProc
	GetProfilePath               ComProc
	GetDefaultDownloadFolderPath ComProc
	PutDefaultDownloadFolderPath ComProc
	GetPreferredColorScheme      ComProc
	PutPreferredColorScheme      ComProc
}

type ICoreWebView2Profile struct {
	vtbl *ICoreWebView2ProfileVtbl
}

type ICoreWebView2Profile6Vtbl struct {
	ICoreWebView2Profile5Vtbl

	GetIsPasswordAutosaveEnabled ComProc
	PutIsPasswordAutosaveEnabled ComProc
	GetIsGeneralAutofillEnabled  ComProc
	PutIsGeneralAutofillEnabled  ComProc
}

type ICoreWebView2Profile6 struct {
	//lint:ignore U1000 campo lido pelo Windows via COM, Go nunca le
	vtbl *ICoreWebView2Profile6Vtbl
}

type ICoreWebView2Profile8Vtbl struct {
	ICoreWebView2Profile7Vtbl

	Delete        ComProc
	AddDeleted    ComProc
	RemoveDeleted ComProc
}

type ICoreWebView2Profile8 struct {
	//lint:ignore U1000 campo lido pelo Windows via COM, Go nunca le
	vtbl *ICoreWebView2Profile8Vtbl
}

type ICoreWebView2WebResourceRequestVtbl struct {
	IUnknownVtbl

	GetURI     ComProc
	PutURI     ComProc
	GetMethod  ComProc
	PutMethod  ComProc
	GetContent ComProc
	PutContent ComProc
	GetHeaders ComProc
}

type ICoreWebView2WebResourceRequest struct {
	vtbl *ICoreWebView2WebResourceRequestVtbl
}

type ICoreWebView2WebResourceRequestedEventArgsVtbl struct {
	IUnknownVtbl

	GetRequest         ComProc
	GetResponse        ComProc
	PutResponse        ComProc
	GetDeferral        ComProc
	GetResourceContext ComProc
}

type ICoreWebView2WebResourceRequestedEventArgs struct {
	vtbl *ICoreWebView2WebResourceRequestedEventArgsVtbl
}

type ICoreWebView2WebResourceResponseVtbl struct {
	IUnknownVtbl

	GetContent      ComProc
	PutContent      ComProc
	GetHeaders      ComProc
	GetStatusCode   ComProc
	PutStatusCode   ComProc
	GetReasonPhrase ComProc
	PutReasonPhrase ComProc
}

type ICoreWebView2WebResourceResponse struct {
	vtbl *ICoreWebView2WebResourceResponseVtbl
}

type ICoreWebView2CompositionController2Vtbl struct {
	ICoreWebView2CompositionControllerVtbl

	GetAutomationProvider ComProc
}

type ICoreWebView2CompositionController2 struct {
	//lint:ignore U1000 campo lido pelo Windows via COM, Go nunca le
	vtbl *ICoreWebView2CompositionController2Vtbl
}

type ICoreWebView2ContextMenuItemCollectionVtbl struct {
	IUnknownVtbl

	GetCount           ComProc
	GetValueAtIndex    ComProc
	RemoveValueAtIndex ComProc
	InsertValueAtIndex ComProc
}

type ICoreWebView2ContextMenuItemCollection struct {
	vtbl *ICoreWebView2ContextMenuItemCollectionVtbl
}

type ICoreWebView2Controller2Vtbl struct {
	ICoreWebView2ControllerVtbl

	GetDefaultBackgroundColor ComProc
	PutDefaultBackgroundColor ComProc
}

type ICoreWebView2Controller2 struct {
	vtbl *ICoreWebView2Controller2Vtbl
}

type ICoreWebView2Controller4Vtbl struct {
	ICoreWebView2Controller3Vtbl

	GetAllowExternalDrop ComProc
	PutAllowExternalDrop ComProc
}

type ICoreWebView2Controller4 struct {
	vtbl *ICoreWebView2Controller4Vtbl
}

type ICoreWebView2ControllerOptions2Vtbl struct {
	ICoreWebView2ControllerOptionsVtbl

	GetScriptLocale ComProc
	PutScriptLocale ComProc
}

type ICoreWebView2ControllerOptions2 struct {
	//lint:ignore U1000 campo lido pelo Windows via COM, Go nunca le
	vtbl *ICoreWebView2ControllerOptions2Vtbl
}

type ICoreWebView2ControllerOptions3Vtbl struct {
	ICoreWebView2ControllerOptions2Vtbl

	GetDefaultBackgroundColor ComProc
	PutDefaultBackgroundColor ComProc
}

type ICoreWebView2ControllerOptions3 struct {
	//lint:ignore U1000 campo lido pelo Windows via COM, Go nunca le
	vtbl *ICoreWebView2ControllerOptions3Vtbl
}

type ICoreWebView2DevToolsProtocolEventReceivedEventArgsVtbl struct {
	IUnknownVtbl

	GetParameterObjectAsJSON ComProc
}

type ICoreWebView2DevToolsProtocolEventReceivedEventArgs struct {
	//lint:ignore U1000 campo lido pelo Windows via COM, Go nunca le
	vtbl *ICoreWebView2DevToolsProtocolEventReceivedEventArgsVtbl
}

type ICoreWebView2Environment10Vtbl struct {
	ICoreWebView2Environment9Vtbl

	CreateCoreWebView2ControllerOptions                ComProc
	CreateCoreWebView2ControllerWithOptions            ComProc
	CreateCoreWebView2CompositionControllerWithOptions ComProc
}

type ICoreWebView2Environment10 struct {
	//lint:ignore U1000 campo lido pelo Windows via COM, Go nunca le
	vtbl *ICoreWebView2Environment10Vtbl
}

type ICoreWebView2Environment11Vtbl struct {
	ICoreWebView2Environment10Vtbl

	GetFailureReportFolderPath ComProc
}

type ICoreWebView2Environment11 struct {
	//lint:ignore U1000 campo lido pelo Windows via COM, Go nunca le
	vtbl *ICoreWebView2Environment11Vtbl
}

type ICoreWebView2Environment12Vtbl struct {
	ICoreWebView2Environment11Vtbl

	CreateSharedBuffer ComProc
}

type ICoreWebView2Environment12 struct {
	//lint:ignore U1000 campo lido pelo Windows via COM, Go nunca le
	vtbl *ICoreWebView2Environment12Vtbl
}

type ICoreWebView2Environment13Vtbl struct {
	ICoreWebView2Environment12Vtbl

	GetProcessExtendedInfos ComProc
}

type ICoreWebView2Environment13 struct {
	//lint:ignore U1000 campo lido pelo Windows via COM, Go nunca le
	vtbl *ICoreWebView2Environment13Vtbl
}

type ICoreWebView2Environment14Vtbl struct {
	ICoreWebView2Environment13Vtbl

	CreateWebFileSystemFileHandle      ComProc
	CreateWebFileSystemDirectoryHandle ComProc
	CreateObjectCollection             ComProc
}

type ICoreWebView2Environment14 struct {
	//lint:ignore U1000 campo lido pelo Windows via COM, Go nunca le
	vtbl *ICoreWebView2Environment14Vtbl
}

type ICoreWebView2Environment15Vtbl struct {
	ICoreWebView2Environment14Vtbl

	CreateFindOptions ComProc
}

type ICoreWebView2Environment15 struct {
	vtbl *ICoreWebView2Environment15Vtbl
}

type ICoreWebView2Environment2Vtbl struct {
	ICoreWebView2EnvironmentVtbl

	CreateWebResourceRequest ComProc
}

type ICoreWebView2Environment2 struct {
	//lint:ignore U1000 campo lido pelo Windows via COM, Go nunca le
	vtbl *ICoreWebView2Environment2Vtbl
}

type ICoreWebView2Environment3Vtbl struct {
	ICoreWebView2Environment2Vtbl

	CreateCoreWebView2CompositionController ComProc
	CreateCoreWebView2PointerInfo           ComProc
}

type ICoreWebView2Environment3 struct {
	//lint:ignore U1000 campo lido pelo Windows via COM, Go nunca le
	vtbl *ICoreWebView2Environment3Vtbl
}

type ICoreWebView2Environment4Vtbl struct {
	ICoreWebView2Environment3Vtbl

	GetAutomationProviderForWindow ComProc
}

type ICoreWebView2Environment4 struct {
	//lint:ignore U1000 campo lido pelo Windows via COM, Go nunca le
	vtbl *ICoreWebView2Environment4Vtbl
}

type ICoreWebView2Environment5Vtbl struct {
	ICoreWebView2Environment4Vtbl

	AddBrowserProcessExited    ComProc
	RemoveBrowserProcessExited ComProc
}

type ICoreWebView2Environment5 struct {
	//lint:ignore U1000 campo lido pelo Windows via COM, Go nunca le
	vtbl *ICoreWebView2Environment5Vtbl
}

type ICoreWebView2Environment6Vtbl struct {
	ICoreWebView2Environment5Vtbl

	CreatePrintSettings ComProc
}

type ICoreWebView2Environment6 struct {
	//lint:ignore U1000 campo lido pelo Windows via COM, Go nunca le
	vtbl *ICoreWebView2Environment6Vtbl
}

type ICoreWebView2Environment7Vtbl struct {
	ICoreWebView2Environment6Vtbl

	GetUserDataFolder ComProc
}

type ICoreWebView2Environment7 struct {
	//lint:ignore U1000 campo lido pelo Windows via COM, Go nunca le
	vtbl *ICoreWebView2Environment7Vtbl
}

type ICoreWebView2Environment8Vtbl struct {
	ICoreWebView2Environment7Vtbl

	AddProcessInfosChanged    ComProc
	RemoveProcessInfosChanged ComProc
	GetProcessInfos           ComProc
}

type ICoreWebView2Environment8 struct {
	//lint:ignore U1000 campo lido pelo Windows via COM, Go nunca le
	vtbl *ICoreWebView2Environment8Vtbl
}

type ICoreWebView2Environment9Vtbl struct {
	ICoreWebView2Environment8Vtbl

	CreateContextMenuItem ComProc
}

type ICoreWebView2Environment9 struct {
	vtbl *ICoreWebView2Environment9Vtbl
}

type ICoreWebView2EnvironmentOptions4Vtbl struct {
	IUnknownVtbl

	GetCustomSchemeRegistrations ComProc
	SetCustomSchemeRegistrations ComProc
}

type ICoreWebView2EnvironmentOptions4 struct {
	//lint:ignore U1000 campo lido pelo Windows via COM, Go nunca le
	vtbl *ICoreWebView2EnvironmentOptions4Vtbl
}

type ICoreWebView2EnvironmentOptions5Vtbl struct {
	IUnknownVtbl

	GetEnableTrackingPrevention ComProc
	PutEnableTrackingPrevention ComProc
}

type ICoreWebView2EnvironmentOptions5 struct {
	//lint:ignore U1000 campo lido pelo Windows via COM, Go nunca le
	vtbl *ICoreWebView2EnvironmentOptions5Vtbl
}

type ICoreWebView2EnvironmentOptions6Vtbl struct {
	IUnknownVtbl

	GetAreBrowserExtensionsEnabled ComProc
	PutAreBrowserExtensionsEnabled ComProc
}

type ICoreWebView2EnvironmentOptions6 struct {
	//lint:ignore U1000 campo lido pelo Windows via COM, Go nunca le
	vtbl *ICoreWebView2EnvironmentOptions6Vtbl
}

type ICoreWebView2CreateCoreWebView2EnvironmentCompletedHandlerVtbl struct {
	IUnknownVtbl

	Invoke ComProc
}

type ICoreWebView2CreateCoreWebView2EnvironmentCompletedHandler struct {
	//lint:ignore U1000 campo lido pelo Windows via COM, Go nunca le
	vtbl *ICoreWebView2CreateCoreWebView2EnvironmentCompletedHandlerVtbl
}

type ICoreWebView2FindStartCompletedHandlerVtbl struct {
	IUnknownVtbl

	Invoke ComProc
}

type ICoreWebView2FindStartCompletedHandler struct {
	//lint:ignore U1000 campo lido pelo Windows via COM, Go nunca le
	vtbl *ICoreWebView2FindStartCompletedHandlerVtbl
}

type ICoreWebView2FindVtbl struct {
	IUnknownVtbl

	GetActiveMatchIndex           ComProc
	GetMatchCount                 ComProc
	AddActiveMatchIndexChanged    ComProc
	RemoveActiveMatchIndexChanged ComProc
	AddMatchCountChanged          ComProc
	RemoveMatchCountChanged       ComProc
	Start                         ComProc
	FindNext                      ComProc
	FindPrevious                  ComProc
	Stop                          ComProc
}

type ICoreWebView2Find struct {
	vtbl *ICoreWebView2FindVtbl
}

type ICoreWebView2FindOptionsVtbl struct {
	IUnknownVtbl

	GetFindTerm                  ComProc
	PutFindTerm                  ComProc
	GetIsCaseSensitive           ComProc
	PutIsCaseSensitive           ComProc
	GetShouldHighlightAllMatches ComProc
	PutShouldHighlightAllMatches ComProc
	GetShouldMatchWord           ComProc
	PutShouldMatchWord           ComProc
	GetSuppressDefaultFindDialog ComProc
	PutSuppressDefaultFindDialog ComProc
}

type ICoreWebView2FindOptions struct {
	vtbl *ICoreWebView2FindOptionsVtbl
}

type ICoreWebView2Frame3Vtbl struct {
	ICoreWebView2Frame2Vtbl

	AddPermissionRequested    ComProc
	RemovePermissionRequested ComProc
}

type ICoreWebView2Frame3 struct {
	//lint:ignore U1000 campo lido pelo Windows via COM, Go nunca le
	vtbl *ICoreWebView2Frame3Vtbl
}

type ICoreWebView2Frame4Vtbl struct {
	ICoreWebView2Frame3Vtbl

	PostSharedBufferToScript ComProc
}

type ICoreWebView2Frame4 struct {
	//lint:ignore U1000 campo lido pelo Windows via COM, Go nunca le
	vtbl *ICoreWebView2Frame4Vtbl
}

type ICoreWebView2Frame5Vtbl struct {
	ICoreWebView2Frame4Vtbl

	GetFrameID ComProc
}

type ICoreWebView2Frame5 struct {
	//lint:ignore U1000 campo lido pelo Windows via COM, Go nunca le
	vtbl *ICoreWebView2Frame5Vtbl
}

type ICoreWebView2Frame6Vtbl struct {
	ICoreWebView2Frame5Vtbl

	AddScreenCaptureStarting    ComProc
	RemoveScreenCaptureStarting ComProc
}

type ICoreWebView2Frame6 struct {
	//lint:ignore U1000 campo lido pelo Windows via COM, Go nunca le
	vtbl *ICoreWebView2Frame6Vtbl
}

type ICoreWebView2Frame7Vtbl struct {
	ICoreWebView2Frame6Vtbl

	AddFrameCreated    ComProc
	RemoveFrameCreated ComProc
}

type ICoreWebView2Frame7 struct {
	//lint:ignore U1000 campo lido pelo Windows via COM, Go nunca le
	vtbl *ICoreWebView2Frame7Vtbl
}

type ICoreWebView2FrameInfoVtbl struct {
	IUnknownVtbl

	GetName   ComProc
	GetSource ComProc
}

type ICoreWebView2FrameInfo struct {
	//lint:ignore U1000 campo lido pelo Windows via COM, Go nunca le
	vtbl *ICoreWebView2FrameInfoVtbl
}

type ICoreWebView2NavigationStartingEventArgs2Vtbl struct {
	ICoreWebView2NavigationStartingEventArgsVtbl

	GetAdditionalAllowedFrameAncestors ComProc
	PutAdditionalAllowedFrameAncestors ComProc
}

type ICoreWebView2NavigationStartingEventArgs2 struct {
	//lint:ignore U1000 campo lido pelo Windows via COM, Go nunca le
	vtbl *ICoreWebView2NavigationStartingEventArgs2Vtbl
}

type ICoreWebView2NewWindowRequestedEventArgs2Vtbl struct {
	ICoreWebView2NewWindowRequestedEventArgsVtbl

	GetName ComProc
}

type ICoreWebView2NewWindowRequestedEventArgs2 struct {
	//lint:ignore U1000 campo lido pelo Windows via COM, Go nunca le
	vtbl *ICoreWebView2NewWindowRequestedEventArgs2Vtbl
}

type ICoreWebView2ObjectCollectionViewVtbl struct {
	IUnknownVtbl

	GetCount        ComProc
	GetValueAtIndex ComProc
}

type ICoreWebView2ObjectCollectionView struct {
	//lint:ignore U1000 campo lido pelo Windows via COM, Go nunca le
	vtbl *ICoreWebView2ObjectCollectionViewVtbl
}

type ICoreWebView2PermissionRequestedEventArgs2Vtbl struct {
	ICoreWebView2PermissionRequestedEventArgsVtbl

	GetHandled ComProc
	PutHandled ComProc
}

type ICoreWebView2PermissionRequestedEventArgs2 struct {
	//lint:ignore U1000 campo lido pelo Windows via COM, Go nunca le
	vtbl *ICoreWebView2PermissionRequestedEventArgs2Vtbl
}

type ICoreWebView2ProcessFailedEventArgsVtbl struct {
	IUnknownVtbl

	GetProcessFailedKind ComProc
}

type ICoreWebView2ProcessFailedEventArgs struct {
	vtbl *ICoreWebView2ProcessFailedEventArgsVtbl
}

type ICoreWebView2ProcessFailedEventArgs3Vtbl struct {
	ICoreWebView2ProcessFailedEventArgs2Vtbl

	GetFailureSourceModulePath ComProc
}

type ICoreWebView2ProcessFailedEventArgs3 struct {
	//lint:ignore U1000 campo lido pelo Windows via COM, Go nunca le
	vtbl *ICoreWebView2ProcessFailedEventArgs3Vtbl
}

type ICoreWebView2ClearBrowsingDataCompletedHandlerVtbl struct {
	IUnknownVtbl

	Invoke ComProc
}

type ICoreWebView2ClearBrowsingDataCompletedHandler struct {
	//lint:ignore U1000 campo lido pelo Windows via COM, Go nunca le
	vtbl *ICoreWebView2ClearBrowsingDataCompletedHandlerVtbl
}

type ICoreWebView2Profile2Vtbl struct {
	ICoreWebView2ProfileVtbl

	ClearBrowsingData            ComProc
	ClearBrowsingDataInTimeRange ComProc
	ClearBrowsingDataAll         ComProc
}

type ICoreWebView2Profile2 struct {
	vtbl *ICoreWebView2Profile2Vtbl
}

type ICoreWebView2Profile3Vtbl struct {
	ICoreWebView2Profile2Vtbl

	GetPreferredTrackingPreventionLevel ComProc
	PutPreferredTrackingPreventionLevel ComProc
}

type ICoreWebView2Profile3 struct {
	//lint:ignore U1000 campo lido pelo Windows via COM, Go nunca le
	vtbl *ICoreWebView2Profile3Vtbl
}

type ICoreWebView2Profile4Vtbl struct {
	ICoreWebView2Profile3Vtbl

	SetPermissionState              ComProc
	GetNonDefaultPermissionSettings ComProc
}

type ICoreWebView2Profile4 struct {
	//lint:ignore U1000 campo lido pelo Windows via COM, Go nunca le
	vtbl *ICoreWebView2Profile4Vtbl
}

type ICoreWebView2Profile5Vtbl struct {
	ICoreWebView2Profile4Vtbl

	GetCookieManager ComProc
}

type ICoreWebView2Profile5 struct {
	//lint:ignore U1000 campo lido pelo Windows via COM, Go nunca le
	vtbl *ICoreWebView2Profile5Vtbl
}

type ICoreWebView2ProfileAddBrowserExtensionCompletedHandlerVtbl struct {
	IUnknownVtbl

	Invoke ComProc
}

type ICoreWebView2ProfileAddBrowserExtensionCompletedHandler struct {
	//lint:ignore U1000 campo lido pelo Windows via COM, Go nunca le
	vtbl *ICoreWebView2ProfileAddBrowserExtensionCompletedHandlerVtbl
}

type ICoreWebView2ProfileGetBrowserExtensionsCompletedHandlerVtbl struct {
	IUnknownVtbl

	Invoke ComProc
}

type ICoreWebView2ProfileGetBrowserExtensionsCompletedHandler struct {
	//lint:ignore U1000 campo lido pelo Windows via COM, Go nunca le
	vtbl *ICoreWebView2ProfileGetBrowserExtensionsCompletedHandlerVtbl
}

type ICoreWebView2BrowserExtensionListVtbl struct {
	IUnknownVtbl

	GetCount        ComProc
	GetValueAtIndex ComProc
}

type ICoreWebView2BrowserExtensionList struct {
	vtbl *ICoreWebView2BrowserExtensionListVtbl
}

type ICoreWebView2Profile7Vtbl struct {
	ICoreWebView2Profile6Vtbl

	AddBrowserExtension  ComProc
	GetBrowserExtensions ComProc
}

type ICoreWebView2Profile7 struct {
	vtbl *ICoreWebView2Profile7Vtbl
}

type ICoreWebView2Settings2Vtbl struct {
	ICoreWebView2SettingsVtbl

	GetUserAgent ComProc
	PutUserAgent ComProc
}

type ICoreWebView2Settings2 struct {
	//lint:ignore U1000 campo lido pelo Windows via COM, Go nunca le
	vtbl *ICoreWebView2Settings2Vtbl
}

type ICoreWebView2Settings3Vtbl struct {
	ICoreWebView2Settings2Vtbl

	GetAreBrowserAcceleratorKeysEnabled ComProc
	PutAreBrowserAcceleratorKeysEnabled ComProc
}

type ICoreWebView2Settings3 struct {
	//lint:ignore U1000 campo lido pelo Windows via COM, Go nunca le
	vtbl *ICoreWebView2Settings3Vtbl
}

type ICoreWebView2Settings5Vtbl struct {
	ICoreWebView2Settings4Vtbl

	GetIsPinchZoomEnabled ComProc
	PutIsPinchZoomEnabled ComProc
}

type ICoreWebView2Settings5 struct {
	//lint:ignore U1000 campo lido pelo Windows via COM, Go nunca le
	vtbl *ICoreWebView2Settings5Vtbl
}

type ICoreWebView2Settings6Vtbl struct {
	ICoreWebView2Settings5Vtbl

	GetIsSwipeNavigationEnabled ComProc
	PutIsSwipeNavigationEnabled ComProc
}

type ICoreWebView2Settings6 struct {
	vtbl *ICoreWebView2Settings6Vtbl
}

type ICoreWebView2Settings7Vtbl struct {
	ICoreWebView2Settings6Vtbl

	GetHiddenPdfToolbarItems ComProc
	PutHiddenPdfToolbarItems ComProc
}

type ICoreWebView2Settings7 struct {
	//lint:ignore U1000 campo lido pelo Windows via COM, Go nunca le
	vtbl *ICoreWebView2Settings7Vtbl
}

type ICoreWebView2Settings8Vtbl struct {
	ICoreWebView2Settings7Vtbl

	GetIsReputationCheckingRequired ComProc
	PutIsReputationCheckingRequired ComProc
}

type ICoreWebView2Settings8 struct {
	//lint:ignore U1000 campo lido pelo Windows via COM, Go nunca le
	vtbl *ICoreWebView2Settings8Vtbl
}

type ICoreWebView2SourceChangedEventArgsVtbl struct {
	IUnknownVtbl

	GetIsNewDocument ComProc
}

type ICoreWebView2SourceChangedEventArgs struct {
	//lint:ignore U1000 campo lido pelo Windows via COM, Go nunca le
	vtbl *ICoreWebView2SourceChangedEventArgsVtbl
}

type ICoreWebView2WebMessageReceivedEventArgsVtbl struct {
	IUnknownVtbl

	GetSource                ComProc
	GetWebMessageAsJSON      ComProc
	TryGetWebMessageAsString ComProc
}

type ICoreWebView2WebMessageReceivedEventArgs struct {
	vtbl *ICoreWebView2WebMessageReceivedEventArgsVtbl
}

type ICoreWebView2_10Vtbl struct {
	ICoreWebView2_9Vtbl

	AddBasicAuthenticationRequested    ComProc
	RemoveBasicAuthenticationRequested ComProc
}

type ICoreWebView2_10 struct {
	//lint:ignore U1000 campo lido pelo Windows via COM, Go nunca le
	vtbl *ICoreWebView2_10Vtbl
}

type ICoreWebView2_13Vtbl struct {
	ICoreWebView2_12Vtbl

	GetProfile ComProc
}

type ICoreWebView2_13 struct {
	vtbl *ICoreWebView2_13Vtbl
}

type ICoreWebView2_14Vtbl struct {
	ICoreWebView2_13Vtbl

	AddServerCertificateErrorDetected    ComProc
	RemoveServerCertificateErrorDetected ComProc
	ClearServerCertificateErrorActions   ComProc
}

type ICoreWebView2_14 struct {
	//lint:ignore U1000 campo lido pelo Windows via COM, Go nunca le
	vtbl *ICoreWebView2_14Vtbl
}

type ICoreWebView2_17Vtbl struct {
	ICoreWebView2_16Vtbl

	PostSharedBufferToScript ComProc
}

type ICoreWebView2_17 struct {
	//lint:ignore U1000 campo lido pelo Windows via COM, Go nunca le
	vtbl *ICoreWebView2_17Vtbl
}

type ICoreWebView2_18Vtbl struct {
	ICoreWebView2_17Vtbl

	AddLaunchingExternalURIScheme    ComProc
	RemoveLaunchingExternalURIScheme ComProc
}

type ICoreWebView2_18 struct {
	//lint:ignore U1000 campo lido pelo Windows via COM, Go nunca le
	vtbl *ICoreWebView2_18Vtbl
}

type ICoreWebView2_19Vtbl struct {
	ICoreWebView2_18Vtbl

	GetMemoryUsageTargetLevel ComProc
	PutMemoryUsageTargetLevel ComProc
}

type ICoreWebView2_19 struct {
	vtbl *ICoreWebView2_19Vtbl
}

type ICoreWebView2_20Vtbl struct {
	ICoreWebView2_19Vtbl

	GetFrameID ComProc
}

type ICoreWebView2_20 struct {
	//lint:ignore U1000 campo lido pelo Windows via COM, Go nunca le
	vtbl *ICoreWebView2_20Vtbl
}

type ICoreWebView2_21Vtbl struct {
	ICoreWebView2_20Vtbl

	ExecuteScriptWithResult ComProc
}

type ICoreWebView2_21 struct {
	//lint:ignore U1000 campo lido pelo Windows via COM, Go nunca le
	vtbl *ICoreWebView2_21Vtbl
}

type ICoreWebView2_22Vtbl struct {
	ICoreWebView2_21Vtbl

	AddWebResourceRequestedFilterWithRequestSourceKinds    ComProc
	RemoveWebResourceRequestedFilterWithRequestSourceKinds ComProc
}

type ICoreWebView2_22 struct {
	//lint:ignore U1000 campo lido pelo Windows via COM, Go nunca le
	vtbl *ICoreWebView2_22Vtbl
}

type ICoreWebView2_25Vtbl struct {
	ICoreWebView2_24Vtbl

	AddSaveAsUIShowing    ComProc
	RemoveSaveAsUIShowing ComProc
	ShowSaveAsUI          ComProc
}

type ICoreWebView2_25 struct {
	//lint:ignore U1000 campo lido pelo Windows via COM, Go nunca le
	vtbl *ICoreWebView2_25Vtbl
}

type ICoreWebView2_23Vtbl struct {
	ICoreWebView2_22Vtbl

	PostWebMessageAsJSONWithAdditionalObjects ComProc
}

type ICoreWebView2_23 struct {
	//lint:ignore U1000 campo lido pelo Windows via COM, Go nunca le
	vtbl *ICoreWebView2_23Vtbl
}

type ICoreWebView2_27Vtbl struct {
	ICoreWebView2_26Vtbl

	AddScreenCaptureStarting    ComProc
	RemoveScreenCaptureStarting ComProc
}

type ICoreWebView2_27 struct {
	//lint:ignore U1000 campo lido pelo Windows via COM, Go nunca le
	vtbl *ICoreWebView2_27Vtbl
}

type ICoreWebView2_28Vtbl struct {
	ICoreWebView2_27Vtbl

	GetFind ComProc
}

type ICoreWebView2_28 struct {
	vtbl *ICoreWebView2_28Vtbl
}

type ICoreWebView2_26Vtbl struct {
	ICoreWebView2_25Vtbl

	AddSaveFileSecurityCheckStarting    ComProc
	RemoveSaveFileSecurityCheckStarting ComProc
}

type ICoreWebView2_26 struct {
	//lint:ignore U1000 campo lido pelo Windows via COM, Go nunca le
	vtbl *ICoreWebView2_26Vtbl
}

type ICoreWebView2_24Vtbl struct {
	ICoreWebView2_23Vtbl

	AddNotificationReceived    ComProc
	RemoveNotificationReceived ComProc
}

type ICoreWebView2_24 struct {
	//lint:ignore U1000 campo lido pelo Windows via COM, Go nunca le
	vtbl *ICoreWebView2_24Vtbl
}

type ICoreWebView2_5Vtbl struct {
	ICoreWebView2_4Vtbl

	AddClientCertificateRequested    ComProc
	RemoveClientCertificateRequested ComProc
}

type ICoreWebView2_5 struct {
	//lint:ignore U1000 campo lido pelo Windows via COM, Go nunca le
	vtbl *ICoreWebView2_5Vtbl
}

type ICoreWebView2_6Vtbl struct {
	ICoreWebView2_5Vtbl

	OpenTaskManagerWindow ComProc
}

type ICoreWebView2_6 struct {
	//lint:ignore U1000 campo lido pelo Windows via COM, Go nunca le
	vtbl *ICoreWebView2_6Vtbl
}

type ICoreWebView2_7Vtbl struct {
	ICoreWebView2_6Vtbl

	PrintToPdf ComProc
}

type ICoreWebView2_7 struct {
	//lint:ignore U1000 campo lido pelo Windows via COM, Go nunca le
	vtbl *ICoreWebView2_7Vtbl
}

type ComProc uintptr

type IUnknownVtbl struct {
	QueryInterface ComProc
	AddRef         ComProc
	Release        ComProc
}

type IUnknown struct {
	vtbl *IUnknownVtbl
}

//exhaustruct:ignore // Espelho da API Win32 (GUID): todos os campos via NewGUID.
type GUID struct {
	Data1 uint32
	Data2 uint16
	Data3 uint16
	Data4 [8]byte
}

type refCounted struct {
	refs int32
	_    int32
	iid  *GUID
}

//nolint:gochecknoglobals // IID raiz do COM: constante de processo.
var IIDIUnknown = NewGUID("{00000000-0000-0000-C000-000000000046}")

//exhaustruct:ignore // Handler COM: refs/iid fixados no construtor logo apos; zero intermediario nao escapa.
type envCompletedHandler struct {
	vtbl *ICoreWebView2CreateCoreWebView2EnvironmentCompletedHandlerVtbl

	refCounted
	cb func(hr uintptr, env *ICoreWebView2Environment) uintptr
}

//exhaustruct:ignore // Handler COM: refs/iid fixados no construtor logo apos; zero intermediario nao escapa.
type controllerCompletedHandler struct {
	vtbl *ICoreWebView2CreateCoreWebView2ControllerCompletedHandlerVtbl

	refCounted
	cb func(hr uintptr, ctrl *ICoreWebView2Controller) uintptr
}

//exhaustruct:ignore // Handler COM: refs/iid fixados no construtor logo apos; zero intermediario nao escapa.
type addExtCompletedHandler struct {
	vtbl *ICoreWebView2ProfileAddBrowserExtensionCompletedHandlerVtbl

	refCounted
	cb func(hr uintptr, ext *ICoreWebView2BrowserExtension) uintptr
}

//exhaustruct:ignore // Handler COM: refs/iid fixados no construtor logo apos; zero intermediario nao escapa.
type getExtCompletedHandler struct {
	vtbl *ICoreWebView2ProfileGetBrowserExtensionsCompletedHandlerVtbl

	refCounted
	cb func(hr uintptr, list *ICoreWebView2BrowserExtensionList) uintptr
}

//exhaustruct:ignore // Handler COM: refs/iid fixados no construtor logo apos; zero intermediario nao escapa.
type removeExtCompletedHandler struct {
	vtbl *ICoreWebView2BrowserExtensionRemoveCompletedHandlerVtbl

	refCounted
	cb func(hr uintptr) uintptr
}

//exhaustruct:ignore // Handler COM: refs/iid fixados no construtor logo apos; zero intermediario nao escapa.
type enableExtCompletedHandler struct {
	vtbl *ICoreWebView2BrowserExtensionEnableCompletedHandlerVtbl

	refCounted
	cb func(hr uintptr) uintptr
}

//exhaustruct:ignore // Handler COM: refs/iid fixados no construtor logo apos; zero intermediario nao escapa.
type schemeReg struct {
	vtbl *ICoreWebView2CustomSchemeRegistrationVtbl

	refCounted
}

//exhaustruct:ignore // Handler COM: refs/iid fixados no construtor logo apos; zero intermediario nao escapa.
type cdpCompletedHandler struct {
	vtbl *ICoreWebView2CallDevToolsProtocolMethodCompletedHandlerVtbl

	refCounted
	cb func(hr uintptr, result string) uintptr
}

//exhaustruct:ignore // Handler COM: refs/iid fixados no construtor logo apos; zero intermediario nao escapa.
type srcChangedHandler struct {
	vtbl *ICoreWebView2SourceChangedEventHandlerVtbl

	refCounted
	cb func(sender *ICoreWebView2) uintptr
}

//exhaustruct:ignore // Handler COM: refs/iid fixados no construtor logo apos; zero intermediario nao escapa.
type navStartingHandler struct {
	vtbl *ICoreWebView2NavigationStartingEventHandlerVtbl

	refCounted
	cb func(sender *ICoreWebView2, args *ICoreWebView2NavigationStartingEventArgs) uintptr
}

//exhaustruct:ignore // Handler COM: refs/iid fixados no construtor logo apos; zero intermediario nao escapa.
type navCompletedHandler struct {
	vtbl *ICoreWebView2NavigationCompletedEventHandlerVtbl

	refCounted
	cb func(sender *ICoreWebView2, args *ICoreWebView2NavigationCompletedEventArgs) uintptr
}

//exhaustruct:ignore // Handler COM: refs/iid fixados no construtor logo apos; zero intermediario nao escapa.
type titleChangedHandler struct {
	vtbl *ICoreWebView2DocumentTitleChangedEventHandlerVtbl

	refCounted
	cb func(sender *ICoreWebView2) uintptr
}

//exhaustruct:ignore // Handler COM: refs/iid fixados no construtor logo apos; zero intermediario nao escapa.
type newWindowHandler struct {
	vtbl *ICoreWebView2NewWindowRequestedEventHandlerVtbl

	refCounted
	cb func(sender *ICoreWebView2, args *ICoreWebView2NewWindowRequestedEventArgs) uintptr
}

//exhaustruct:ignore // Handler COM: refs/iid fixados no construtor logo apos; zero intermediario nao escapa.
type webMessageHandler struct {
	vtbl *ICoreWebView2WebMessageReceivedEventHandlerVtbl

	refCounted
	cb func(sender *ICoreWebView2, args *ICoreWebView2WebMessageReceivedEventArgs) uintptr
}

//exhaustruct:ignore // Handler COM: refs/iid fixados no construtor logo apos; zero intermediario nao escapa.
type accelKeyHandler struct {
	vtbl *ICoreWebView2AcceleratorKeyPressedEventHandlerVtbl

	refCounted
	cb func(sender *ICoreWebView2Controller, args *ICoreWebView2AcceleratorKeyPressedEventArgs) uintptr
}

//exhaustruct:ignore // Handler COM: refs/iid fixados no construtor logo apos; zero intermediario nao escapa.
type clearBrowsingDataHandler struct {
	vtbl *ICoreWebView2ClearBrowsingDataCompletedHandlerVtbl

	refCounted
	cb func(hr uintptr) uintptr
}

//exhaustruct:ignore // Handler COM: refs/iid fixados no construtor logo apos; zero intermediario nao escapa.
type findStartHandler struct {
	vtbl *ICoreWebView2FindStartCompletedHandlerVtbl

	refCounted
}

func findStartInvoke(_ *findStartHandler, _ uintptr) uintptr {
	return SOK
}

func newFindStartHandler() *findStartHandler {
	h := &findStartHandler{vtbl: &findStartVtbl}
	h.refs = 1
	h.iid = IIDICoreWebView2FindStartCompletedHandler

	return h
}

//exhaustruct:ignore // Handler COM: refs/iid fixados no construtor logo apos; zero intermediario nao escapa.
type webResourceRequestedHandler struct {
	vtbl *ICoreWebView2WebResourceRequestedEventHandlerVtbl

	refCounted
	cb func(sender *ICoreWebView2, args *ICoreWebView2WebResourceRequestedEventArgs) uintptr
}

//exhaustruct:ignore // Handler COM: refs/iid fixados no construtor logo apos; zero intermediario nao escapa.
type contextMenuRequestedHandler struct {
	vtbl *ICoreWebView2ContextMenuRequestedEventHandlerVtbl

	refCounted
	cb func(sender *ICoreWebView2, args *ICoreWebView2ContextMenuRequestedEventArgs) uintptr
}

//exhaustruct:ignore // Handler COM: refs/iid fixados no construtor logo apos; zero intermediario nao escapa.
type customItemSelectedHandler struct {
	vtbl *ICoreWebView2CustomItemSelectedEventHandlerVtbl

	refCounted
	cb func(sender *ICoreWebView2ContextMenuItem, args *IUnknown) uintptr
}

//exhaustruct:ignore // Handler COM: refs/iid fixados no construtor logo apos; zero intermediario nao escapa.
type downloadStartingHandler struct {
	vtbl *ICoreWebView2DownloadStartingEventHandlerVtbl

	refCounted
	cb func(sender *ICoreWebView2, args *ICoreWebView2DownloadStartingEventArgs) uintptr
}

//exhaustruct:ignore // Handler COM: refs/iid fixados no construtor logo apos; zero intermediario nao escapa.
type downloadBytesHandler struct {
	vtbl *ICoreWebView2BytesReceivedChangedEventHandlerVtbl

	refCounted
	cb func(sender *ICoreWebView2DownloadOperation, args *IUnknown) uintptr
}

//exhaustruct:ignore // Handler COM: refs/iid fixados no construtor logo apos; zero intermediario nao escapa.
type downloadStateHandler struct {
	vtbl *ICoreWebView2StateChangedEventHandlerVtbl

	refCounted
	cb func(sender *ICoreWebView2DownloadOperation, args *IUnknown) uintptr
}

//exhaustruct:ignore // Handler COM: refs/iid fixados no construtor logo apos; zero intermediario nao escapa.
type isMutedChangedHandler struct {
	vtbl *ICoreWebView2IsMutedChangedEventHandlerVtbl

	refCounted
	cb func(sender *ICoreWebView2, args *IUnknown) uintptr
}

//exhaustruct:ignore // Handler COM: refs/iid fixados no construtor logo apos; zero intermediario nao escapa.
type isPlayingAudioChangedHandler struct {
	vtbl *ICoreWebView2IsDocumentPlayingAudioChangedEventHandlerVtbl

	refCounted
	cb func(sender *ICoreWebView2, args *IUnknown) uintptr
}

//exhaustruct:ignore // Handler COM: refs/iid fixados no construtor logo apos; zero intermediario nao escapa.
type permissionRequestedHandler struct {
	vtbl *ICoreWebView2PermissionRequestedEventHandlerVtbl

	refCounted
	cb func(sender *ICoreWebView2, args *ICoreWebView2PermissionRequestedEventArgs) uintptr
}

//exhaustruct:ignore // Handler COM: refs/iid fixados no construtor logo apos; zero intermediario nao escapa.
type processFailedHandler struct {
	vtbl *ICoreWebView2ProcessFailedEventHandlerVtbl

	refCounted
	cb func(sender *ICoreWebView2, args *ICoreWebView2ProcessFailedEventArgs) uintptr
}

//exhaustruct:ignore // Handler COM: refs/iid fixados no construtor logo apos; zero intermediario nao escapa.
type gotFocusHandler struct {
	vtbl *ICoreWebView2FocusChangedEventHandlerVtbl

	refCounted
	cb func() uintptr
}

//exhaustruct:ignore // Handler COM: refs/iid fixados no construtor logo apos; zero intermediario nao escapa.
type evalCompletedHandler struct {
	vtbl *ICoreWebView2ExecuteScriptCompletedHandlerVtbl

	refCounted
	cb func(hr uintptr, result *uint16) uintptr
}

//exhaustruct:ignore // Handler COM: refs/iid fixados no construtor logo apos; zero intermediario nao escapa.
type trySuspendHandler struct {
	vtbl *ICoreWebView2TrySuspendCompletedHandlerVtbl

	refCounted
	cb func(hr uintptr, success uintptr) uintptr
}

//exhaustruct:ignore // Handler COM: refs/iid fixados no construtor logo apos; zero intermediario nao escapa.
type saveAsUIHandler struct {
	vtbl *ICoreWebView2ShowSaveAsUICompletedHandlerVtbl

	refCounted
	cb func(hr uintptr, resultado uintptr) uintptr
}

//exhaustruct:ignore // Handler COM: refs fixado no construtor logo apos; zero intermediario nao escapa.
type optSub struct {
	vtbl  unsafe.Pointer
	refs  int32
	_     int32
	owner *EnvironmentOptions
}

//exhaustruct:ignore // Opcoes de ambiente WebView2: construtor parcial por desenho.
type EnvironmentOptions struct {
	base optSub
	sub4 optSub
	sub5 optSub
	sub6 optSub
	refs int32

	liberado bool

	AdditionalBrowserArguments             string
	Language                               string
	TargetCompatibleBrowserVersion         string
	AllowSingleSignOnUsingOSPrimaryAccount bool
	AreBrowserExtensionsEnabled            bool
	EnableTrackingPrevention               *bool

	schemeReg *schemeReg
}

type Environment struct {
	ptr            *ICoreWebView2Environment
	BrowserVersion string
}

//exhaustruct:ignore // Opcoes de criacao do ambiente: so campos usados; resto padrao do WebView2.
type CreateOptions struct {
	BrowserExecutableFolder        string
	UserDataFolder                 string
	AdditionalBrowserArguments     string
	Language                       string
	TargetCompatibleBrowserVersion string
	EnableExtensions               bool
	EnableTrackingPrevention       *bool
}

//exhaustruct:ignore // Janela WebView2: construtor parcial por desenho (campos sob ciclo de vida).
type WebView struct {
	ctrl *ICoreWebView2Controller
	view *ICoreWebView2

	ctrl2  *ICoreWebView2Controller2
	ctrl3  *ICoreWebView2Controller3
	ctrl4  *ICoreWebView2Controller4
	view3  *ICoreWebView2_3
	view4  *ICoreWebView2_4
	view8  *ICoreWebView2_8
	view11 *ICoreWebView2_11
	view13 *ICoreWebView2_13

	view19 *ICoreWebView2_19
	view25 *ICoreWebView2_25
	view28 *ICoreWebView2_28

	ownerEnv *Environment

	settings *ICoreWebView2Settings
	set6     *ICoreWebView2Settings6
	profile  *ICoreWebView2Profile7

	AllowPermissions  bool
	AllowExternalDrop bool
	memoryLow         bool

	extIDs   map[string]string
	extID    string
	extPopup string

	extPopups map[string]string

	OnContextAction    func(id, value string) bool
	OnDownloadStarting func(args *ICoreWebView2DownloadStartingEventArgs)
	OnAudioChanged     func(playing bool)
	OnMutedChanged     func(muted bool)
	OnTitleChanged     func(string)

	OnPermissionRequested func(uri string, kind uint32) bool

	OnNavigationStarting func(uri string)

	OnSourceChanged func(uri string)
	OnNavigated     func(success bool)
	OnNewWindow     func(uri string, userInitiated bool) bool
	OnAccelKey      func(vk uint32, ctrl bool, shift bool, alt bool) bool
	OnProcessFail   func(kind uint32)
	OnMessage       func(string)
	OnGotFocus      func()

	Diag string
}

func NewComProc(fn any) ComProc { return ComProc(windows.NewCallback(fn)) }

//nolint:unparam // Assinatura exigida pela ABI (SyscallN retorna 3 valores); chamadores ignoram o que nao usam.
func (p ComProc) Call(args ...uintptr) (uintptr, uintptr, error) {
	r1, r2, errno := syscall.SyscallN(uintptr(p), args...)
	if errno != 0 { // safe-ignore: errno fresco de SyscallN, nunca embrulhado; !=0 testa "houve erro"
		return r1, r2, erroTextoF("COM: %w", errno)
	}

	return r1, r2, nil
}

func Failed(hr uintptr) bool { return int32(uint32(hr)) < 0 }

func hrErr(hr uintptr, what string) error {
	if Failed(hr) {
		return erroTextoF("%s falhou: HRESULT 0x%08X", what, uint32(hr))
	}

	return nil
}

func NewGUID(s string) *GUID {
	g, err := parseGUID(s)
	if err != nil {
		panic("wv2: GUID invalido: " + s)
	}

	return g
}

func parseGUID(texto string) (*GUID, error) {
	const tamanhoGUIDTexto = 32

	textoLimpo := strings.TrimSpace(texto)
	textoLimpo = strings.TrimPrefix(textoLimpo, "{")
	textoLimpo = strings.TrimSuffix(textoLimpo, "}")

	textoLimpo = strings.ReplaceAll(textoLimpo, "-", "")
	if len(textoLimpo) != tamanhoGUIDTexto {
		return nil, erroTextoF("wv2: GUID invalido: %s", texto)
	}

	decodificado, err := hex.DecodeString(textoLimpo)
	if err != nil {
		return nil, erroTextoF("wv2: GUID invalido: %s", texto)
	}

	const deslocamentoByteGUID = 8

	resultado := &GUID{}
	resultado.Data1 = uint32(decodificado[0])<<24 | uint32(decodificado[1])<<16 |
		uint32(decodificado[2])<<deslocamentoByteGUID | uint32(decodificado[3])
	resultado.Data2 = uint16(decodificado[4])<<deslocamentoByteGUID | uint16(decodificado[5])
	resultado.Data3 = uint16(decodificado[6])<<deslocamentoByteGUID | uint16(decodificado[7])
	copy(resultado.Data4[:], decodificado[8:16])

	return resultado, nil
}

func vtableOf(this unsafe.Pointer) unsafe.Pointer {
	return *(*unsafe.Pointer)(this)
}

func queryInterface(this unsafe.Pointer, iid *GUID) unsafe.Pointer {
	if this == nil {
		return nil
	}

	vt := (*IUnknownVtbl)(vtableOf(this))

	var out unsafe.Pointer
	vt.QueryInterface.Call(uintptr(this), uintptr(unsafe.Pointer(iid)), uintptr(unsafe.Pointer(&out)))

	return out
}

func b2u(b bool) uintptr {
	if b {
		return 1
	}

	return 0
}

func InitCOM() error {
	r, _, _ := procCoInitializeEx.Call(0, coinitApartmentThreaded)
	if Failed(r) && uint32(r) != 0x80010106 {
		return hrErr(r, "CoInitializeEx")
	}

	return nil
}

func coTaskMemFree(p unsafe.Pointer) {
	if p != nil {
		procCoTaskMemFree.Call(uintptr(p))
	}
}

func utf16Ptr(s string) *uint16 {
	p, err := windows.UTF16PtrFromString(s)
	if err != nil {
		return nil
	}

	return p
}

func takeString(ponteiro *uint16) string {
	if ponteiro == nil {
		tracef("takeString: ponteiro nulo")

		return ""
	}

	tracef("takeString: lendo %#x", uintptr(unsafe.Pointer(ponteiro)))
	s := windows.UTF16PtrToString(ponteiro)
	tracef("takeString: leu %d chars, liberando", len(s))
	coTaskMemFree(unsafe.Pointer(ponteiro))
	tracef("takeString: liberou ok")

	return s
}

func readString(ponteiro *uint16) string {
	if ponteiro == nil {
		tracef("readString: ponteiro nulo")

		return ""
	}

	tracef("readString: copiando (sem liberar) %#x", uintptr(unsafe.Pointer(ponteiro)))
	s := windows.UTF16PtrToString(ponteiro)
	tracef("readString: copiou %d chars", len(s))

	return s
}

func allocString(s string) *uint16 {
	unidades, err := windows.UTF16FromString(s)
	if err != nil {
		unidades = []uint16{0}
	}

	const bytesPorUnidadeUTF16WV = 2

	buf, _, _ := procCoTaskMemAlloc.Call(uintptr(len(unidades)) * bytesPorUnidadeUTF16WV)
	if buf == 0 {
		return nil
	}

	dst := unsafe.Slice((*uint16)(unsafe.Pointer(buf)), len(unidades))
	copy(dst, unidades)

	return (*uint16)(unsafe.Pointer(buf))
}

func headerOf(this unsafe.Pointer) *refCounted {
	return (*refCounted)(unsafe.Add(this, refsOffset))
}

func guidEqual(a, b *GUID) bool {
	if a == nil || b == nil {
		return false
	}

	return a.Data1 == b.Data1 && a.Data2 == b.Data2 && a.Data3 == b.Data3 && a.Data4 == b.Data4
}

func unkQueryInterface(this unsafe.Pointer, refiid uintptr, obj uintptr) uintptr {
	if obj == 0 {
		return EInvalidArg
	}

	iid := (*GUID)(unsafe.Pointer(refiid))

	cabecalho := headerOf(this)
	if iid == nil || guidEqual(iid, IIDIUnknown) || guidEqual(iid, cabecalho.iid) {
		tracef("QI this=%#x pediu %s -> SOK", this, GUIDString(iid))

		atomic.AddInt32(&cabecalho.refs, 1)

		*(*unsafe.Pointer)(unsafe.Pointer(obj)) = this

		return SOK
	}

	tracef("QI this=%#x pediu %s (nosso=%s) -> ENoInterface", this, GUIDString(iid), GUIDString(cabecalho.iid))

	return ENoInterface
}

func unkAddRef(this unsafe.Pointer) uintptr {
	return uintptr(atomic.AddInt32(&headerOf(this).refs, 1))
}

func unkRelease(this unsafe.Pointer) uintptr {
	restante := uintptr(atomic.AddInt32(&headerOf(this).refs, -1))

	if restante == 0 {
		unpin(uintptr(this))
	}

	return restante
}

func tracef(format string, args ...any) {
	if TraceHook != nil {
		TraceHook(fmt.Sprintf(format, args...))
	}
}

func GUIDString(identificador *GUID) string {
	if identificador == nil {
		return "(nil)"
	}

	return fmt.Sprintf("{%08X-%04X-%04X-%02X%02X-%02X%02X%02X%02X%02X%02X}",
		identificador.Data1, identificador.Data2, identificador.Data3,
		identificador.Data4[0], identificador.Data4[1], identificador.Data4[2], identificador.Data4[3],
		identificador.Data4[4], identificador.Data4[5], identificador.Data4[6], identificador.Data4[7])
}

func guard(where string) {
	if r := recover(); r != nil {
		msg := fmt.Sprintf("PANICO em %s: %v", where, r)
		if PanicHook != nil {
			PanicHook(msg)
		}
	}
}

func unkTriple() IUnknownVtbl {
	return IUnknownVtbl{
		NewComProc(unkQueryInterface),
		NewComProc(unkAddRef),
		NewComProc(unkRelease),
	}
}

func envCompletedInvoke(this *envCompletedHandler, hr uintptr, env *ICoreWebView2Environment) uintptr {
	defer guard("EnvironmentCompleted")

	if this.cb != nil {
		return this.cb(hr, env)
	}

	return SOK
}

func newEnvCompletedHandler(cb func(uintptr, *ICoreWebView2Environment) uintptr) *envCompletedHandler {
	h := &envCompletedHandler{vtbl: &envCompletedVtbl, cb: cb}
	h.refs = 1
	h.iid = IIDICoreWebView2CreateCoreWebView2EnvironmentCompletedHandler

	return h
}

func controllerCompletedInvoke(this *controllerCompletedHandler, hr uintptr, ctrl *ICoreWebView2Controller) uintptr {
	defer guard("ControllerCompleted")

	if this.cb != nil {
		return this.cb(hr, ctrl)
	}

	return SOK
}

func newControllerCompletedHandler(cb func(uintptr, *ICoreWebView2Controller) uintptr) *controllerCompletedHandler {
	h := &controllerCompletedHandler{vtbl: &controllerCompletedVtbl, cb: cb}
	h.refs = 1
	h.iid = IIDICoreWebView2CreateCoreWebView2ControllerCompletedHandler

	return h
}

func addExtCompletedInvoke(this *addExtCompletedHandler, hr uintptr, ext *ICoreWebView2BrowserExtension) uintptr {
	defer guard("AddBrowserExtensionCompleted")

	if this.cb != nil {
		return this.cb(hr, ext)
	}

	return SOK
}

func newAddExtCompletedHandler(cb func(uintptr, *ICoreWebView2BrowserExtension) uintptr) *addExtCompletedHandler {
	h := &addExtCompletedHandler{vtbl: &addExtCompletedVtbl, cb: cb}
	h.refs = 1
	h.iid = IIDICoreWebView2ProfileAddBrowserExtensionCompletedHandler

	return h
}

func getExtCompletedInvoke(this *getExtCompletedHandler, hr uintptr, list *ICoreWebView2BrowserExtensionList) uintptr {
	defer guard("GetBrowserExtensionsCompleted")

	if this.cb != nil {
		return this.cb(hr, list)
	}

	return SOK
}

func newGetExtCompletedHandler(cb func(uintptr, *ICoreWebView2BrowserExtensionList) uintptr) *getExtCompletedHandler {
	h := &getExtCompletedHandler{vtbl: &getExtCompletedVtbl, cb: cb}
	h.refs = 1
	h.iid = IIDICoreWebView2ProfileGetBrowserExtensionsCompletedHandler

	return h
}

func removeExtCompletedInvoke(this *removeExtCompletedHandler, hr uintptr) uintptr {
	defer guard("RemoveBrowserExtensionCompleted")

	if this.cb != nil {
		return this.cb(hr)
	}

	return SOK
}

func newRemoveExtCompletedHandler(cb func(uintptr) uintptr) *removeExtCompletedHandler {
	h := &removeExtCompletedHandler{vtbl: &removeExtCompletedVtbl, cb: cb}
	h.refs = 1
	h.iid = IIDICoreWebView2BrowserExtensionRemoveCompletedHandler

	return h
}

func enableExtCompletedInvoke(this *enableExtCompletedHandler, hr uintptr) uintptr {
	defer guard("EnableBrowserExtensionCompleted")

	if this.cb != nil {
		return this.cb(hr)
	}

	return SOK
}

func newEnableExtCompletedHandler(cb func(uintptr) uintptr) *enableExtCompletedHandler {
	h := &enableExtCompletedHandler{vtbl: &enableExtCompletedVtbl, cb: cb}
	h.refs = 1
	h.iid = IIDICoreWebView2BrowserExtensionEnableCompletedHandler

	return h
}

func cdpCompletedInvoke(this *cdpCompletedHandler, hr uintptr, result *uint16) uintptr {
	tracef("CDP.Invoke ENTROU this=%p hr=%#x result=%#x", this, hr, result)

	defer guard("CallDevToolsProtocolMethodCompleted")

	if this.cb != nil {
		s := readString(result)

		tracef("CDP.Invoke: chamando o callback do Go")

		r := this.cb(hr, s)
		tracef("CDP.Invoke: callback retornou %#x", r)

		return r
	}

	tracef("CDP.Invoke: sem callback")

	return SOK
}

func newCDPCompletedHandler(cb func(uintptr, string) uintptr) *cdpCompletedHandler {
	h := &cdpCompletedHandler{vtbl: &cdpCompletedVtbl, cb: cb}
	h.refs = 1
	h.iid = IIDICoreWebView2CallDevToolsProtocolMethodCompletedHandler

	return h
}

func srcChangedInvoke(this *srcChangedHandler, sender *ICoreWebView2, _ *ICoreWebView2SourceChangedEventArgs) uintptr {
	defer guard("SourceChanged")

	if this.cb != nil {
		return this.cb(sender)
	}

	return SOK
}

func newSrcChangedHandler(cb func(*ICoreWebView2) uintptr) *srcChangedHandler {
	h := &srcChangedHandler{vtbl: &srcChangedVtbl, cb: cb}
	h.refs = 1
	h.iid = IIDICoreWebView2SourceChangedEventHandler

	return h
}

func navStartingInvoke(this *navStartingHandler, sender *ICoreWebView2,
	args *ICoreWebView2NavigationStartingEventArgs,
) uintptr {
	defer guard("NavigationStarting")

	if this.cb != nil {
		return this.cb(sender, args)
	}

	return SOK
}

func newNavStartingHandler(cb func(*ICoreWebView2,
	*ICoreWebView2NavigationStartingEventArgs) uintptr,
) *navStartingHandler {
	h := &navStartingHandler{vtbl: &navStartingVtbl, cb: cb}
	h.refs = 1
	h.iid = IIDICoreWebView2NavigationStartingEventHandler

	return h
}

func navCompletedInvoke(this *navCompletedHandler, sender *ICoreWebView2,
	args *ICoreWebView2NavigationCompletedEventArgs,
) uintptr {
	defer guard("NavigationCompleted")

	if this.cb != nil {
		return this.cb(sender, args)
	}

	return SOK
}

func newNavCompletedHandler(cb func(*ICoreWebView2,
	*ICoreWebView2NavigationCompletedEventArgs) uintptr,
) *navCompletedHandler {
	h := &navCompletedHandler{vtbl: &navCompletedVtbl, cb: cb}
	h.refs = 1
	h.iid = IIDICoreWebView2NavigationCompletedEventHandler

	return h
}

func titleChangedInvoke(this *titleChangedHandler, sender *ICoreWebView2, _ *IUnknown) uintptr {
	defer guard("DocumentTitleChanged")

	if this.cb != nil {
		return this.cb(sender)
	}

	return SOK
}

func newTitleChangedHandler(cb func(*ICoreWebView2) uintptr) *titleChangedHandler {
	h := &titleChangedHandler{vtbl: &titleChangedVtbl, cb: cb}
	h.refs = 1
	h.iid = IIDICoreWebView2DocumentTitleChangedEventHandler

	return h
}

func newWindowInvoke(this *newWindowHandler, sender *ICoreWebView2,
	args *ICoreWebView2NewWindowRequestedEventArgs,
) uintptr {
	defer guard("NewWindowRequested")

	if this.cb != nil {
		return this.cb(sender, args)
	}

	return SOK
}

func newNewWindowHandler(cb func(*ICoreWebView2, *ICoreWebView2NewWindowRequestedEventArgs) uintptr) *newWindowHandler {
	h := &newWindowHandler{vtbl: &newWindowVtbl, cb: cb}
	h.refs = 1
	h.iid = IIDICoreWebView2NewWindowRequestedEventHandler

	return h
}

func webMessageInvoke(this *webMessageHandler, sender *ICoreWebView2,
	args *ICoreWebView2WebMessageReceivedEventArgs,
) uintptr {
	defer guard("WebMessageReceived")

	if this.cb != nil {
		return this.cb(sender, args)
	}

	return SOK
}

func newWebMessageHandler(cb func(*ICoreWebView2,
	*ICoreWebView2WebMessageReceivedEventArgs) uintptr,
) *webMessageHandler {
	h := &webMessageHandler{vtbl: &webMessageVtbl, cb: cb}
	h.refs = 1
	h.iid = IIDICoreWebView2WebMessageReceivedEventHandler

	return h
}

func accelKeyInvoke(this *accelKeyHandler, sender *ICoreWebView2Controller,
	args *ICoreWebView2AcceleratorKeyPressedEventArgs,
) uintptr {
	defer guard("AcceleratorKeyPressed")

	if this.cb != nil {
		return this.cb(sender, args)
	}

	return SOK
}

func newAccelKeyHandler(cb func(*ICoreWebView2Controller,
	*ICoreWebView2AcceleratorKeyPressedEventArgs) uintptr,
) *accelKeyHandler {
	h := &accelKeyHandler{vtbl: &accelKeyVtbl, cb: cb}
	h.refs = 1
	h.iid = IIDICoreWebView2AcceleratorKeyPressedEventHandler

	return h
}

func clearBrowsingDataInvoke(this *clearBrowsingDataHandler, hr uintptr) uintptr {
	defer guard("ClearBrowsingData")

	if this.cb != nil {
		return this.cb(hr)
	}

	return SOK
}

func newClearBrowsingDataHandler(cb func(uintptr) uintptr) *clearBrowsingDataHandler {
	h := &clearBrowsingDataHandler{vtbl: &clearBrowsingDataVtbl, cb: cb}
	h.refs = 1
	h.iid = IIDICoreWebView2ClearBrowsingDataCompletedHandler

	return h
}

func webResourceRequestedInvoke(this *webResourceRequestedHandler, sender *ICoreWebView2,
	args *ICoreWebView2WebResourceRequestedEventArgs,
) uintptr {
	defer guard("WebResourceRequested")

	if this.cb != nil {
		return this.cb(sender, args)
	}

	return SOK
}

func newWebResourceRequestedHandler(cb func(*ICoreWebView2,
	*ICoreWebView2WebResourceRequestedEventArgs) uintptr,
) *webResourceRequestedHandler {
	h := &webResourceRequestedHandler{vtbl: &webResourceRequestedVtbl, cb: cb}
	h.refs = 1
	h.iid = IIDICoreWebView2WebResourceRequestedEventHandler

	return h
}

func contextMenuRequestedInvoke(this *contextMenuRequestedHandler, sender *ICoreWebView2,
	args *ICoreWebView2ContextMenuRequestedEventArgs,
) uintptr {
	defer guard("ContextMenuRequested")

	if this.cb != nil {
		return this.cb(sender, args)
	}

	return SOK
}

func newContextMenuRequestedHandler(cb func(*ICoreWebView2,
	*ICoreWebView2ContextMenuRequestedEventArgs) uintptr,
) *contextMenuRequestedHandler {
	h := &contextMenuRequestedHandler{vtbl: &contextMenuRequestedVtbl, cb: cb}
	h.refs = 1
	h.iid = IIDICoreWebView2ContextMenuRequestedEventHandler

	return h
}

func customItemSelectedInvoke(this *customItemSelectedHandler, sender *ICoreWebView2ContextMenuItem,
	args *IUnknown,
) uintptr {
	defer guard("CustomItemSelected")

	if this.cb != nil {
		return this.cb(sender, args)
	}

	return SOK
}

func newCustomItemSelectedHandler(cb func(*ICoreWebView2ContextMenuItem,
	*IUnknown) uintptr,
) *customItemSelectedHandler {
	h := &customItemSelectedHandler{vtbl: &customItemSelectedVtbl, cb: cb}
	h.refs = 1
	h.iid = IIDICoreWebView2CustomItemSelectedEventHandler

	return h
}

func downloadStartingInvoke(this *downloadStartingHandler, sender *ICoreWebView2,
	args *ICoreWebView2DownloadStartingEventArgs,
) uintptr {
	defer guard("DownloadStarting")

	if this.cb != nil {
		return this.cb(sender, args)
	}

	return SOK
}

func newDownloadStartingHandler(cb func(*ICoreWebView2,
	*ICoreWebView2DownloadStartingEventArgs) uintptr,
) *downloadStartingHandler {
	h := &downloadStartingHandler{vtbl: &downloadStartingVtbl, cb: cb}
	h.refs = 1
	h.iid = IIDICoreWebView2DownloadStartingEventHandler

	return h
}

func downloadBytesInvoke(this *downloadBytesHandler, sender *ICoreWebView2DownloadOperation, args *IUnknown) uintptr {
	defer guard("BytesReceivedChanged")

	if this.cb != nil {
		return this.cb(sender, args)
	}

	return SOK
}

func newDownloadBytesHandler(cb func(*ICoreWebView2DownloadOperation, *IUnknown) uintptr) *downloadBytesHandler {
	h := &downloadBytesHandler{vtbl: &downloadBytesVtbl, cb: cb}
	h.refs = 1
	h.iid = IIDICoreWebView2BytesReceivedChangedEventHandler

	return h
}

func downloadStateInvoke(this *downloadStateHandler, sender *ICoreWebView2DownloadOperation, args *IUnknown) uintptr {
	defer guard("DownloadStateChanged")

	if this.cb != nil {
		return this.cb(sender, args)
	}

	return SOK
}

func newDownloadStateHandler(cb func(*ICoreWebView2DownloadOperation, *IUnknown) uintptr) *downloadStateHandler {
	h := &downloadStateHandler{vtbl: &downloadStateVtbl, cb: cb}
	h.refs = 1
	h.iid = IIDICoreWebView2StateChangedEventHandler

	return h
}

func isMutedChangedInvoke(this *isMutedChangedHandler, sender *ICoreWebView2, args *IUnknown) uintptr {
	defer guard("IsMutedChanged")

	if this.cb != nil {
		return this.cb(sender, args)
	}

	return SOK
}

func newIsMutedChangedHandler(cb func(*ICoreWebView2, *IUnknown) uintptr) *isMutedChangedHandler {
	h := &isMutedChangedHandler{vtbl: &isMutedChangedVtbl, cb: cb}
	h.refs = 1
	h.iid = IIDICoreWebView2IsMutedChangedEventHandler

	return h
}

func isPlayingAudioChangedInvoke(this *isPlayingAudioChangedHandler, sender *ICoreWebView2, args *IUnknown) uintptr {
	defer guard("IsDocumentPlayingAudioChanged")

	if this.cb != nil {
		return this.cb(sender, args)
	}

	return SOK
}

func newIsDocumentPlayingAudioChangedHandler(cb func(*ICoreWebView2, *IUnknown) uintptr) *isPlayingAudioChangedHandler {
	h := &isPlayingAudioChangedHandler{vtbl: &isPlayingAudioChangedVtbl, cb: cb}
	h.refs = 1
	h.iid = IIDICoreWebView2IsDocumentPlayingAudioChangedEventHandler

	return h
}

func permissionRequestedInvoke(this *permissionRequestedHandler, sender *ICoreWebView2,
	args *ICoreWebView2PermissionRequestedEventArgs,
) uintptr {
	defer guard("PermissionRequested")

	if this.cb != nil {
		return this.cb(sender, args)
	}

	return SOK
}

func newPermissionRequestedHandler(cb func(*ICoreWebView2,
	*ICoreWebView2PermissionRequestedEventArgs) uintptr,
) *permissionRequestedHandler {
	h := &permissionRequestedHandler{vtbl: &permissionRequestedVtbl, cb: cb}
	h.refs = 1
	h.iid = IIDICoreWebView2PermissionRequestedEventHandler

	return h
}

func processFailedInvoke(this *processFailedHandler, sender *ICoreWebView2,
	args *ICoreWebView2ProcessFailedEventArgs,
) uintptr {
	defer guard("ProcessFailed")

	if this.cb != nil {
		return this.cb(sender, args)
	}

	return SOK
}

func newProcessFailedHandler(cb func(*ICoreWebView2,
	*ICoreWebView2ProcessFailedEventArgs) uintptr,
) *processFailedHandler {
	h := &processFailedHandler{vtbl: &processFailedVtbl, cb: cb}
	h.refs = 1
	h.iid = IIDICoreWebView2ProcessFailedEventHandler

	return h
}

func gotFocusInvoke(this *gotFocusHandler, _ *ICoreWebView2Controller, _ *IUnknown) uintptr {
	defer guard("GotFocus")

	if this.cb != nil {
		return this.cb()
	}

	return SOK
}

func newGotFocusHandler(cb func() uintptr) *gotFocusHandler {
	h := &gotFocusHandler{vtbl: &gotFocusVtbl, cb: cb}
	h.refs = 1
	h.iid = IIDICoreWebView2FocusChangedEventHandler

	return h
}

func evalCompletedInvoke(this *evalCompletedHandler, hr uintptr, result *uint16) uintptr {
	defer guard("ExecuteScriptCompleted")

	if this.cb != nil {
		return this.cb(hr, result)
	}

	return SOK
}

func newEvalCompletedHandler(cb func(uintptr, *uint16) uintptr) *evalCompletedHandler {
	h := &evalCompletedHandler{vtbl: &evalCompletedVtbl, cb: cb}
	h.refs = 1
	h.iid = IIDICoreWebView2ExecuteScriptCompletedHandler

	return h
}

func trySuspendCompletedInvoke(this *trySuspendHandler, hr uintptr, success uintptr) uintptr {
	defer guard("TrySuspendCompleted")

	if this.cb != nil {
		cb := this.cb
		this.cb = nil

		return cb(hr, success)
	}

	return SOK
}

func newTrySuspendHandler(cb func(uintptr, uintptr) uintptr) *trySuspendHandler {
	h := &trySuspendHandler{vtbl: &trySuspendVtbl, cb: cb}
	h.refs = 1
	h.iid = IIDICoreWebView2TrySuspendCompletedHandler

	return h
}

func saveAsUICompletedInvoke(this *saveAsUIHandler, hr uintptr, resultado uintptr) uintptr {
	defer guard("ShowSaveAsUICompleted")

	if this.cb != nil {
		cb := this.cb
		this.cb = nil

		return cb(hr, resultado)
	}

	return SOK
}

func newSaveAsUIHandler(cb func(uintptr, uintptr) uintptr) *saveAsUIHandler {
	h := &saveAsUIHandler{vtbl: &saveAsUICVtbl, cb: cb}
	h.refs = 1
	h.iid = IIDICoreWebView2ShowSaveAsUICompletedHandler

	return h
}

func pin(valor any) uintptr {
	rv := reflect.ValueOf(valor)
	if !rv.IsValid() || rv.Kind() != reflect.Pointer || rv.IsNil() {
		return 0
	}

	key := rv.Pointer()

	pinnedMu.Lock()
	pinned[key] = valor
	pinnedMu.Unlock()

	return key
}

func unpin(key uintptr) {
	if key == 0 {
		return
	}

	pinnedMu.Lock()
	delete(pinned, key)
	pinnedMu.Unlock()
}

// NewEnvironmentOptions monta as opcoes de ambiente do WebView2.
//
//odlint:pin-fixo // schemeReg fixado aqui vive enquanto opcoes (processo todo); liberar roda no fim.
func NewEnvironmentOptions(extraArgs, language, targetVersion string, extensions bool,
	trackingPrevention *bool,
) *EnvironmentOptions {
	opcoes := &EnvironmentOptions{
		AdditionalBrowserArguments:     extraArgs,
		Language:                       language,
		TargetCompatibleBrowserVersion: targetVersion,
		AreBrowserExtensionsEnabled:    extensions,
		EnableTrackingPrevention:       trackingPrevention,
	}
	opcoes.refs = 1
	opcoes.base = optSub{vtbl: unsafe.Pointer(&optBaseVtbl), owner: opcoes}
	opcoes.sub4 = optSub{vtbl: unsafe.Pointer(&opt4Vtbl), owner: opcoes}
	opcoes.sub5 = optSub{vtbl: unsafe.Pointer(&opt5Vtbl), owner: opcoes}
	opcoes.sub6 = optSub{vtbl: unsafe.Pointer(&opt6Vtbl), owner: opcoes}
	opcoes.schemeReg = newSchemeReg()
	pin(opcoes.schemeReg)

	return opcoes
}

func (o *EnvironmentOptions) ifaceFor(iid *GUID) unsafe.Pointer {
	switch {
	case guidEqual(iid, IIDIUnknown),
		guidEqual(iid, IIDICoreWebView2EnvironmentOptions):
		return unsafe.Pointer(&o.base)
	case guidEqual(iid, IIDICoreWebView2EnvironmentOptions4):
		return unsafe.Pointer(&o.sub4)
	case guidEqual(iid, IIDICoreWebView2EnvironmentOptions5):
		if o.EnableTrackingPrevention == nil {
			return nil
		}

		return unsafe.Pointer(&o.sub5)
	case guidEqual(iid, IIDICoreWebView2EnvironmentOptions6):
		return unsafe.Pointer(&o.sub6)
	}

	return nil
}

func optQI(this *optSub, refiid uintptr, obj uintptr) uintptr {
	if obj == 0 {
		return EInvalidArg
	}

	ponteiro := this.owner.ifaceFor((*GUID)(unsafe.Pointer(refiid)))
	if ponteiro == nil {
		return ENoInterface
	}

	atomic.AddInt32(&this.owner.refs, 1)

	*(*unsafe.Pointer)(unsafe.Pointer(obj)) = ponteiro

	return SOK
}

func optAddRef(this *optSub) uintptr { return uintptr(atomic.AddInt32(&this.owner.refs, 1)) }

func optRelease(this *optSub) uintptr {
	n := atomic.AddInt32(&this.owner.refs, -1)
	if n == 0 {
		unpin(uintptr(unsafe.Pointer(this.owner)))
	}

	return uintptr(n)
}

func optGetArgs(this *optSub, out **uint16) uintptr {
	if out != nil {
		*out = allocString(this.owner.AdditionalBrowserArguments)
	}

	return SOK
}

func optPutArgs(this *optSub, v *uint16) uintptr {
	this.owner.AdditionalBrowserArguments = windows.UTF16PtrToString(v)

	return SOK
}

func optGetLang(this *optSub, out **uint16) uintptr {
	if out != nil {
		*out = allocString(this.owner.Language)
	}

	return SOK
}

func optPutLang(this *optSub, v *uint16) uintptr {
	this.owner.Language = windows.UTF16PtrToString(v)

	return SOK
}

func optGetTarget(this *optSub, out **uint16) uintptr {
	if out != nil {
		*out = allocString(this.owner.TargetCompatibleBrowserVersion)
	}

	return SOK
}

func optPutTarget(this *optSub, v *uint16) uintptr {
	this.owner.TargetCompatibleBrowserVersion = windows.UTF16PtrToString(v)

	return SOK
}

func optGetSSO(this *optSub, out *int32) uintptr {
	if out != nil {
		*out = boolI32(this.owner.AllowSingleSignOnUsingOSPrimaryAccount)
	}

	return SOK
}

func optPutSSO(this *optSub, v int32) uintptr {
	this.owner.AllowSingleSignOnUsingOSPrimaryAccount = v != 0

	return SOK
}

func opt6GetExt(this *optSub, out *int32) uintptr {
	if out != nil {
		*out = boolI32(this.owner.AreBrowserExtensionsEnabled)
	}

	return SOK
}

func opt6PutExt(this *optSub, v int32) uintptr {
	this.owner.AreBrowserExtensionsEnabled = v != 0

	return SOK
}

func schemeGetName(_ *schemeReg, out **uint16) uintptr {
	if out != nil {
		*out = allocString("overdrive")
	}

	return SOK
}

func schemeGetSecure(_ *schemeReg, out *int32) uintptr {
	if out != nil {
		*out = 1
	}

	return SOK
}

func schemePutSecure(_ *schemeReg, _ int32) uintptr { return SOK }

func schemeGetOrigins(_ *schemeReg, count *uint32, _ ***uint16) uintptr {
	if count != nil {
		*count = 0
	}

	return SOK
}

func schemeSetOrigins(_ *schemeReg, _ uint32, _ **uint16) uintptr { return SOK }

func schemeGetAuthority(_ *schemeReg, out *int32) uintptr {
	if out != nil {
		*out = 1
	}

	return SOK
}

func schemePutAuthority(_ *schemeReg, _ int32) uintptr { return SOK }

func newSchemeReg() *schemeReg {
	h := &schemeReg{vtbl: &schemeRegVtbl}
	h.refs = 1
	h.iid = IIDICoreWebView2CustomSchemeRegistration

	return h
}

func opt4GetSchemes(this *optSub, count *uint32, arr ***ICoreWebView2CustomSchemeRegistration) uintptr {
	reg := this.owner.schemeReg
	if reg == nil {
		if count != nil {
			*count = 0
		}

		return SOK
	}

	buf, _, _ := procCoTaskMemAlloc.Call(unsafe.Sizeof(uintptr(0)))
	if buf == 0 {
		return EFail
	}

	atomic.AddInt32(&reg.refs, 1)
	*(*unsafe.Pointer)(unsafe.Pointer(buf)) = unsafe.Pointer(reg)

	if count != nil {
		*count = 1
	}

	if arr != nil {
		*arr = (**ICoreWebView2CustomSchemeRegistration)(unsafe.Pointer(buf))
	}

	return SOK
}

func opt4SetSchemes(_ *optSub, _ uint32, _ *ICoreWebView2CustomSchemeRegistration) uintptr {
	return SOK
}

func opt5GetTracking(this *optSub, out *int32) uintptr {
	if out != nil {
		*out = boolI32(this.owner.EnableTrackingPrevention != nil && *this.owner.EnableTrackingPrevention)
	}

	return SOK
}

func opt5PutTracking(this *optSub, v int32) uintptr {
	b := v != 0
	this.owner.EnableTrackingPrevention = &b

	return SOK
}

func boolI32(b bool) int32 {
	if b {
		return 1
	}

	return 0
}

func optUnkTriple() IUnknownVtbl {
	return IUnknownVtbl{
		NewComProc(optQI),
		NewComProc(optAddRef),
		NewComProc(optRelease),
	}
}

// NewEnvironmentOptions cria as opcoes de ambiente; o inicial cai em liberar()
// e o resto segue o refcount do COM.
//
//odlint:pin-fixo schemeReg vive enquanto o EnvironmentOptions: a referencia
func (o *EnvironmentOptions) ptr() uintptr { return uintptr(unsafe.Pointer(&o.base)) }

// liberar solta as referencias iniciais mantidas por nos depois que o
// WebView2 terminou de ler as opcoes. Sem isso o objeto (e o schemeReg)
// ficaria pinado para sempre no mapa global.
func (o *EnvironmentOptions) liberar() {
	if o == nil || o.liberado {
		return
	}

	o.liberado = true
	if o.schemeReg != nil {
		sr := o.schemeReg
		o.schemeReg = nil

		unkRelease(unsafe.Pointer(sr))
	}

	optRelease(&o.base)
}

func colorBytes(alfa, verm, verde, azul byte) uint32 {
	const (
		deslocamentoCorVerde = 8
		deslocamentoCorAzul  = 16
		deslocamentoCorAlfa  = 24
	)

	return uint32(alfa) | uint32(verm)<<deslocamentoCorVerde |
		uint32(verde)<<deslocamentoCorAzul | uint32(azul)<<deslocamentoCorAlfa
}

const componenteOpacoCor = 255

func ColorBlack() uint32 { return colorBytes(componenteOpacoCor, 0, 0, 0) }
func ColorWhite() uint32 {
	return colorBytes(componenteOpacoCor, componenteOpacoCor, componenteOpacoCor, componenteOpacoCor)
}

func CreateEnvironment(opcoes CreateOptions, onReady func(*Environment, error)) error {
	var executavel, dadosUsuario *uint16
	if opcoes.BrowserExecutableFolder != "" {
		executavel = utf16Ptr(opcoes.BrowserExecutableFolder)
	}

	if opcoes.UserDataFolder != "" {
		dadosUsuario = utf16Ptr(opcoes.UserDataFolder)
	}

	opts := NewEnvironmentOptions(
		opcoes.AdditionalBrowserArguments, opcoes.Language, opcoes.TargetCompatibleBrowserVersion,
		opcoes.EnableExtensions, opcoes.EnableTrackingPrevention)

	tratador := newEnvCompletedHandler(func(hr uintptr, env *ICoreWebView2Environment) uintptr {
		opts.liberar()

		if Failed(hr) {
			onReady(nil, hrErr(hr, "criar ambiente WebView2"))

			return SOK
		}

		var textoVersao *uint16
		env.vtbl.GetBrowserVersionString.Call(
			uintptr(unsafe.Pointer(env)),
			uintptr(unsafe.Pointer(&textoVersao)),
		)

		env.vtbl.AddRef.Call(uintptr(unsafe.Pointer(env)))
		onReady(&Environment{ptr: env, BrowserVersion: takeString(textoVersao)}, nil)

		return SOK
	})

	pin(opts)
	pin(tratador)

	hr, err := webviewloader.CreateCoreWebView2EnvironmentWithOptions(
		executavel, dadosUsuario, opts.ptr(), uintptr(unsafe.Pointer(tratador)),
	)

	tratador.vtbl.Release.Call(uintptr(unsafe.Pointer(tratador)))

	if err != nil {
		opts.liberar()
		unpin(uintptr(unsafe.Pointer(tratador)))

		return erroTextoF("WebView2Loader: %w", err)
	}

	if Failed(hr) {
		opts.liberar()
		unpin(uintptr(unsafe.Pointer(tratador)))
	}

	return hrErr(hr, "CreateCoreWebView2EnvironmentWithOptions")
}

func (e *Environment) CreateController(parent uintptr, onReady func(*WebView, error)) error {
	tratador := newControllerCompletedHandler(func(hr uintptr, ctrl *ICoreWebView2Controller) uintptr {
		if Failed(hr) {
			onReady(nil, hrErr(hr, "criar controlador WebView2"))

			return SOK
		}

		var view *ICoreWebView2

		hrView, _, _ := ctrl.vtbl.GetCoreWebView2.Call(
			uintptr(unsafe.Pointer(ctrl)),
			uintptr(unsafe.Pointer(&view)),
		)

		if Failed(hrView) || view == nil {
			ctrl.vtbl.Close.Call(uintptr(unsafe.Pointer(ctrl)))
			onReady(nil, hrErr(hrView, "obter ICoreWebView2"))

			return SOK
		}

		ctrl.vtbl.AddRef.Call(uintptr(unsafe.Pointer(ctrl)))
		w := &WebView{ctrl: ctrl, view: view}
		w.resolve()
		onReady(w, nil)

		return SOK
	})
	pin(tratador)

	hr, _, _ := e.ptr.vtbl.CreateCoreWebView2Controller.Call(
		uintptr(unsafe.Pointer(e.ptr)),
		parent,
		uintptr(unsafe.Pointer(tratador)),
	)
	tratador.vtbl.Release.Call(uintptr(unsafe.Pointer(tratador)))

	if Failed(hr) {
		unpin(uintptr(unsafe.Pointer(tratador)))
	}

	return hrErr(hr, "CreateCoreWebView2Controller")
}

func (e *Environment) Close() {
	if e == nil || e.ptr == nil {
		return
	}

	e.ptr.vtbl.Release.Call(uintptr(unsafe.Pointer(e.ptr)))
	e.ptr = nil
}

func extrairURIRequisicao(args *ICoreWebView2WebResourceRequestedEventArgs) (string, bool) {
	var req *ICoreWebView2WebResourceRequest

	hr, _, _ := args.vtbl.GetRequest.Call(uintptr(unsafe.Pointer(args)), uintptr(unsafe.Pointer(&req)))
	if Failed(hr) || req == nil {
		return "", false
	}

	var up *uint16
	req.vtbl.GetURI.Call(uintptr(unsafe.Pointer(req)), uintptr(unsafe.Pointer(&up)))
	uri := takeString(up)

	req.vtbl.Release.Call(uintptr(unsafe.Pointer(req)))

	return uri, true
}

func contextoRequisicaoOD(args *ICoreWebView2WebResourceRequestedEventArgs) (uint32, bool) {
	const (
		contextoFolhaEstiloWV2 = 1
		contextoImagemWV2      = 2
		contextoMidiaWV2       = 3
		contextoScriptWV2      = 5
		contextoXHRWV2         = 6
	)

	var ctx uint32

	args.vtbl.GetResourceContext.Call(uintptr(unsafe.Pointer(args)), uintptr(unsafe.Pointer(&ctx)))

	switch ctx {
	case contextoFolhaEstiloWV2, contextoImagemWV2, contextoMidiaWV2, contextoScriptWV2, contextoXHRWV2:
		return ctx, true
	default:
		return 0, false
	}
}

func ambienteMenuContexto(w *WebView) *Environment {
	menuEnv := w.ownerEnv
	if menuEnv == nil && theApp != nil {
		menuEnv = theApp.env
	}

	return menuEnv
}

func pegaTextoAlvo(target *ICoreWebView2ContextMenuTarget, metodo ComProc) string {
	var ponteiroTexto *uint16

	if metodo == 0 {
		return ""
	}

	metodo.Call(uintptr(unsafe.Pointer(target)), uintptr(unsafe.Pointer(&ponteiroTexto)))

	return takeString(ponteiroTexto)
}

func extrairTextosAlvoContexto(target *ICoreWebView2ContextMenuTarget) (string, string, string) {
	var temLink, temFonte, temSelecao int32
	target.vtbl.GetHasLinkURI.Call(uintptr(unsafe.Pointer(target)), uintptr(unsafe.Pointer(&temLink)))
	target.vtbl.GetHasSourceURI.Call(uintptr(unsafe.Pointer(target)), uintptr(unsafe.Pointer(&temFonte)))
	target.vtbl.GetHasSelection.Call(uintptr(unsafe.Pointer(target)), uintptr(unsafe.Pointer(&temSelecao)))

	link, fonte, selecao := "", "", ""
	if temLink != 0 {
		link = pegaTextoAlvo(target, target.vtbl.GetLinkURI)
	}

	if temFonte != 0 {
		fonte = pegaTextoAlvo(target, target.vtbl.GetSourceURI)
	}

	if temSelecao != 0 {
		selecao = pegaTextoAlvo(target, target.vtbl.GetSelectionText)
	}

	return link, fonte, selecao
}

func abrirAmbienteContexto9(menuEnv *Environment) (*ICoreWebView2Environment9, bool) {
	env9p := queryInterface(unsafe.Pointer(menuEnv.ptr), IIDICoreWebView2Environment9)
	if env9p == nil {
		return nil, false
	}

	return (*ICoreWebView2Environment9)(env9p), true
}

func coletarItensMenuContexto(
	args *ICoreWebView2ContextMenuRequestedEventArgs,
) (*ICoreWebView2ContextMenuItemCollection, bool) {
	var itens *ICoreWebView2ContextMenuItemCollection

	hr, _, _ := args.vtbl.GetMenuItems.Call(uintptr(unsafe.Pointer(args)), uintptr(unsafe.Pointer(&itens)))
	if Failed(hr) || itens == nil {
		return nil, false
	}

	return itens, true
}

func indiceSalvarImagemContexto(itens *ICoreWebView2ContextMenuItemCollection) int {
	var total uint32
	itens.vtbl.GetCount.Call(uintptr(unsafe.Pointer(itens)), uintptr(unsafe.Pointer(&total)))

	for idx := range total {
		var item *ICoreWebView2ContextMenuItem

		hr, _, _ := itens.vtbl.GetValueAtIndex.Call(uintptr(unsafe.Pointer(itens)), uintptr(idx),
			uintptr(unsafe.Pointer(&item)))
		if Failed(hr) || item == nil {
			continue
		}

		rot := ""

		var lp *uint16

		hrL, _, _ := item.vtbl.GetLabel.Call(uintptr(unsafe.Pointer(item)), uintptr(unsafe.Pointer(&lp)))
		if !Failed(hrL) {
			rot = takeString(lp)
		}

		item.vtbl.Release.Call(uintptr(unsafe.Pointer(item)))

		baixo := strings.ToLower(rot)
		if strings.Contains(baixo, "salvar imagem") || strings.Contains(baixo, "save image") {
			return int(idx)
		}
	}

	return -1
}

type adicionadorItemContexto func(label, id, valor string)

func (w *WebView) SetBounds(r Rect) {
	w.ctrl.vtbl.PutBounds.Call(uintptr(unsafe.Pointer(w.ctrl)), uintptr(unsafe.Pointer(&r)))
}

// SetZoomFactor aplica o zoom nativo do controlador: o Chromium mapeia a
// entrada sozinho, sem o deslocamento do zoom via CSS no root.
func (w *WebView) SetZoomFactor(z float64) {
	if w == nil || w.ctrl == nil {
		return
	}

	w.ctrl.vtbl.PutZoomFactor.Call(uintptr(unsafe.Pointer(w.ctrl)), uintptr(math.Float64bits(z)))
}

func (w *WebView) SetVisible(v bool) {
	w.ctrl.vtbl.PutIsVisible.Call(uintptr(unsafe.Pointer(w.ctrl)), b2u(v))
}

func (w *WebView) Focus() {
	w.ctrl.vtbl.MoveFocus.Call(uintptr(unsafe.Pointer(w.ctrl)), MoveFocusProgrammatic)
}

func (w *WebView) SetMuted(v bool) {
	if w == nil || w.view8 == nil {
		return
	}

	w.view8.vtbl.PutIsMuted.Call(uintptr(unsafe.Pointer(w.view8)), b2u(v))
}

func (w *WebView) IsMuted() bool {
	if w == nil || w.view8 == nil {
		return false
	}

	var m int32
	w.view8.vtbl.GetIsMuted.Call(uintptr(unsafe.Pointer(w.view8)), uintptr(unsafe.Pointer(&m)))

	return m != 0
}

func (w *WebView) SetBackgroundColor(c uint32) {
	if w.ctrl2 != nil {
		w.ctrl2.vtbl.PutDefaultBackgroundColor.Call(uintptr(unsafe.Pointer(w.ctrl2)), uintptr(c))
	}
}

func (w *WebView) SetRawPixelBounds() {
	if w.ctrl3 != nil {
		w.ctrl3.vtbl.PutBoundsMode.Call(uintptr(unsafe.Pointer(w.ctrl3)), BoundsModeRawPixels)
		w.ctrl3.vtbl.PutShouldDetectMonitorScaleChanges.Call(uintptr(unsafe.Pointer(w.ctrl3)), 0)
	}
}

func (w *WebView) DisableExternalDrop() {
	w.SetExternalDropEnabled(false)
}

func (w *WebView) SetExternalDropEnabled(v bool) {
	if w.ctrl4 != nil {
		w.ctrl4.vtbl.PutAllowExternalDrop.Call(uintptr(unsafe.Pointer(w.ctrl4)), b2u(v))
	}
}

func (w *WebView) ClearBrowsingDataAll(done func()) {
	if w == nil || w.profile == nil {
		if done != nil {
			done()
		}

		return
	}

	perfil := queryInterface(unsafe.Pointer(w.profile), IIDICoreWebView2Profile2)
	if perfil == nil {
		if done != nil {
			done()
		}

		return
	}

	perfil2 := (*ICoreWebView2Profile2)(perfil)
	defer perfil2.vtbl.Release.Call(uintptr(unsafe.Pointer(perfil2)))

	tratador := newClearBrowsingDataHandler(func(uintptr) uintptr {
		if done != nil {
			done()
		}

		return SOK
	})
	pin(tratador)
	perfil2.vtbl.ClearBrowsingDataAll.Call(uintptr(unsafe.Pointer(perfil2)), uintptr(unsafe.Pointer(tratador)))
	tratador.vtbl.Release.Call(uintptr(unsafe.Pointer(tratador)))
}

func (w *WebView) Close() {
	if w == nil {
		return
	}

	if w.ctrl != nil {
		w.ctrl.vtbl.Close.Call(uintptr(unsafe.Pointer(w.ctrl)))
	}

	w.liberarVisoes()
	w.liberarControladoresEAjustes()
	w.liberarNucleo()
	w.limparCallbacks()
}

func (w *WebView) Navigate(url string) {
	u := utf16Ptr(url)
	w.view.vtbl.Navigate.Call(uintptr(unsafe.Pointer(w.view)), uintptr(unsafe.Pointer(u)))
}

func (w *WebView) NavigateToString(html string) {
	h := utf16Ptr(html)
	w.view.vtbl.NavigateToString.Call(uintptr(unsafe.Pointer(w.view)), uintptr(unsafe.Pointer(h)))
}

func (w *WebView) Reload() {
	w.view.vtbl.Reload.Call(uintptr(unsafe.Pointer(w.view)))
}

func (w *WebView) Stop() {
	w.view.vtbl.Stop.Call(uintptr(unsafe.Pointer(w.view)))
}

func (w *WebView) GoBack() {
	if w == nil || w.view == nil {
		return
	}

	w.view.vtbl.GoBack.Call(uintptr(unsafe.Pointer(w.view)))
}

// CanGoBack diz se ha pagina anterior no historico.
func (w *WebView) CanGoBack() bool {
	if w == nil || w.view == nil {
		return false
	}

	var v int32

	hr, _, _ := w.view.vtbl.GetCanGoBack.Call(uintptr(unsafe.Pointer(w.view)), uintptr(unsafe.Pointer(&v)))

	return !Failed(hr) && v != 0
}

// CanGoForward diz se ha proxima pagina no historico.
func (w *WebView) CanGoForward() bool {
	if w == nil || w.view == nil {
		return false
	}

	var v int32

	hr, _, _ := w.view.vtbl.GetCanGoForward.Call(uintptr(unsafe.Pointer(w.view)), uintptr(unsafe.Pointer(&v)))

	return !Failed(hr) && v != 0
}

func (w *WebView) GoForward() {
	if w == nil || w.view == nil {
		return
	}

	w.view.vtbl.GoForward.Call(uintptr(unsafe.Pointer(w.view)))
}

func (w *WebView) Source() string {
	if w == nil || w.view == nil {
		return ""
	}

	var p *uint16
	w.view.vtbl.GetSource.Call(uintptr(unsafe.Pointer(w.view)), uintptr(unsafe.Pointer(&p)))

	return takeString(p)
}

func (w *WebView) OpenDevTools() {
	if w == nil || w.view == nil {
		return
	}

	w.view.vtbl.OpenDevToolsWindow.Call(uintptr(unsafe.Pointer(w.view)))
}

func (w *WebView) ExecuteScript(js string) {
	if w == nil || w.view == nil {
		return
	}

	s := utf16Ptr(js)
	w.view.vtbl.ExecuteScript.Call(uintptr(unsafe.Pointer(w.view)), uintptr(unsafe.Pointer(s)), 0)
}

func (w *WebView) EvalJS(script string, retorno func(err error, resultJSON string)) {
	if w == nil || w.view == nil {
		if retorno != nil {
			retorno(erroTexto("sem WebView"), "")
		}

		return
	}

	tratador := newEvalCompletedHandler(func(hr uintptr, result *uint16) uintptr {
		if Failed(hr) {
			retorno(hrErr(hr, "ExecuteScript"), "")

			return SOK
		}

		retorno(nil, readString(result))

		return SOK
	})
	pin(tratador)

	s := utf16Ptr(script)
	hr, _, _ := w.view.vtbl.ExecuteScript.Call(
		uintptr(unsafe.Pointer(w.view)),
		uintptr(unsafe.Pointer(s)),
		uintptr(unsafe.Pointer(tratador)),
	)

	tratador.vtbl.Release.Call(uintptr(unsafe.Pointer(tratador)))

	if Failed(hr) {
		retorno(hrErr(hr, "ExecuteScript"), "")
	}
}

func (w *WebView) AddInitScript(js string) {
	if w == nil || w.view == nil {
		return
	}

	s := utf16Ptr(js)
	w.view.vtbl.AddScriptToExecuteOnDocumentCreated.Call(
		uintptr(unsafe.Pointer(w.view)), uintptr(unsafe.Pointer(s)), 0,
	)
}

func (w *WebView) CallDevTools(method, paramsJSON string) {
	if w == nil || w.view == nil {
		return
	}

	metodo := utf16Ptr(method)

	parametros := utf16Ptr(paramsJSON)
	if parametros == nil {
		parametros = utf16Ptr("{}")
	}

	w.view.vtbl.CallDevToolsProtocolMethod.Call(
		uintptr(unsafe.Pointer(w.view)),
		uintptr(unsafe.Pointer(metodo)),
		uintptr(unsafe.Pointer(parametros)),
		0,
	)
}

func (w *WebView) CallDevToolsResult(method, paramsJSON string, retorno func(err error, resultJSON string)) {
	if w == nil || w.view == nil {
		if retorno != nil {
			retorno(erroTexto("sem WebView"), "")
		}

		return
	}

	tratador := newCDPCompletedHandler(func(hr uintptr, result string) uintptr {
		tracef("CallDevToolsResult: callback do handler (%d bytes, hr=%#x)", len(result), hr)

		if Failed(hr) {
			retorno(hrErr(hr, "CallDevToolsProtocolMethod"), result)

			return SOK
		}

		retorno(nil, result)

		return SOK
	})
	pin(tratador)

	metodo := utf16Ptr(method)

	parametros := utf16Ptr(paramsJSON)
	if parametros == nil {
		parametros = utf16Ptr("{}")
	}

	tracef("CallDevToolsResult: chamando %s (handler=%p)", method, tratador)
	hr, _, _ := w.view.vtbl.CallDevToolsProtocolMethod.Call(
		uintptr(unsafe.Pointer(w.view)),
		uintptr(unsafe.Pointer(metodo)),
		uintptr(unsafe.Pointer(parametros)),
		uintptr(unsafe.Pointer(tratador)),
	)

	tratador.vtbl.Release.Call(uintptr(unsafe.Pointer(tratador)))
	tracef("CallDevToolsResult: %s retornou hr=%#x", method, hr)

	if Failed(hr) {
		retorno(hrErr(hr, "CallDevToolsProtocolMethod"), "")
	}
}

func (w *WebView) TrySuspend() {
	w.setMemoryLow(true)

	if w.view3 == nil {
		return
	}

	var key uintptr

	tratador := newTrySuspendHandler(func(uintptr, uintptr) uintptr {
		if key != 0 {
			unpin(key)
			key = 0
		}

		return SOK
	})
	key = pin(tratador)
	hr, _, _ := w.view3.vtbl.TrySuspend.Call(uintptr(unsafe.Pointer(w.view3)), uintptr(unsafe.Pointer(tratador)))
	tratador.vtbl.Release.Call(uintptr(unsafe.Pointer(tratador)))

	if Failed(hr) {
		unpin(key)
		key = 0
	}
}

func (w *WebView) IsSuspended() bool {
	if w.view3 == nil {
		return false
	}

	var v int32

	hr, _, _ := w.view3.vtbl.GetIsSuspended.Call(uintptr(unsafe.Pointer(w.view3)), uintptr(unsafe.Pointer(&v)))

	return !Failed(hr) && v != 0
}

// MostrarSalvarComo abre o dialogo nativo de salvar pagina completa.
func (w *WebView) MostrarSalvarComo() bool {
	if w == nil || w.view25 == nil {
		return false
	}

	var key uintptr

	tratador := newSaveAsUIHandler(func(uintptr, uintptr) uintptr {
		if key != 0 {
			unpin(key)
			key = 0
		}

		return SOK
	})
	key = pin(tratador)
	hr, _, _ := w.view25.vtbl.ShowSaveAsUI.Call(uintptr(unsafe.Pointer(w.view25)), uintptr(unsafe.Pointer(tratador)))
	tratador.vtbl.Release.Call(uintptr(unsafe.Pointer(tratador)))

	if Failed(hr) {
		unpin(key)
		key = 0

		return false
	}

	return true
}

func (w *WebView) SetScriptsEnabled(on bool) {
	if w == nil || w.view == nil {
		return
	}

	if params, err := cdpScriptDesligado(!on); err == nil {
		w.CallDevTools("Emulation.setScriptExecutionDisabled", params)
	}
}

func (w *WebView) SetSaveData() {
	if w == nil || w.view == nil {
		return
	}

	w.CallDevTools("Network.enable", "{}")
	w.CallDevTools("Network.setExtraHTTPHeaders", `{"headers":{"Save-Data":"on"}}`)
}

func (w *WebView) SetBlockedFonts() {
	if w == nil || w.view == nil {
		return
	}

	w.CallDevTools("Network.enable", "{}")
	w.CallDevTools("Network.setBlockedURLs", `{"urls":["*fonts.googleapis.com/*",`+
		`"*fonts.gstatic.com/*","*use.typekit.net/*","*fonts.bunny.net/*"]}`)
}

// motionReduce e o valor de prefers-reduced-motion que desliga animacao.
const motionReduce = "reduce"

func (w *WebView) SetReducedMotion(ligado bool) {
	if w == nil || w.view == nil {
		return
	}

	motion := idNoPreference
	dados := idNoPreference
	transp := idNoPreference

	if ligado {
		motion = motionReduce
		dados = motionReduce
		transp = motionReduce
	}

	w.CallDevTools("Emulation.setEmulatedMedia", fmt.Sprintf(`{"media":"screen",`+
		`"features":[{"name":"prefers-reduced-motion","value":%q},`+
		`{"name":"prefers-reduced-data","value":%q},`+
		`{"name":"prefers-reduced-transparency","value":%q}]}`,
		motion, dados, transp))
}

func (w *WebView) SetFrozen(frozen bool) {
	if w == nil || w.view == nil {
		return
	}

	w.CallDevToolsResult("Emulation.setScriptExecutionDisabled", fmt.Sprintf(`{"value":%t}`, frozen), func(error, string) {
	})
}

func (w *WebView) StartFind(options *ICoreWebView2FindOptions) bool {
	if w == nil || w.view28 == nil || options == nil {
		return false
	}

	var find *ICoreWebView2Find

	hr, _, _ := w.view28.vtbl.GetFind.Call(uintptr(unsafe.Pointer(w.view28)), uintptr(unsafe.Pointer(&find)))
	if Failed(hr) || find == nil {
		return false
	}

	tratador := newFindStartHandler()
	pin(tratador)
	hr, _, _ = find.vtbl.Start.Call(uintptr(unsafe.Pointer(find)), uintptr(unsafe.Pointer(options)),
		uintptr(unsafe.Pointer(tratador)))
	find.vtbl.Release.Call(uintptr(unsafe.Pointer(find)))
	tratador.vtbl.Release.Call(uintptr(unsafe.Pointer(tratador)))

	if Failed(hr) {
		unpin(uintptr(unsafe.Pointer(tratador)))

		return false
	}

	return true
}

func (w *WebView) Resume() {
	if w.view3 != nil {
		w.view3.vtbl.Resume.Call(uintptr(unsafe.Pointer(w.view3)))
	}

	w.setMemoryLow(false)
}

func (w *WebView) SetDevToolsEnabled(v bool) {
	if w.settings == nil {
		return
	}

	w.settings.vtbl.PutAreDevToolsEnabled.Call(uintptr(unsafe.Pointer(w.settings)), b2u(v))
}

func (w *WebView) SetContextMenuEnabled(v bool) {
	if w.settings == nil {
		return
	}

	w.settings.vtbl.PutAreDefaultContextMenusEnabled.Call(uintptr(unsafe.Pointer(w.settings)), b2u(v))
}

func (w *WebView) SetStatusBarEnabled(v bool) {
	if w.settings == nil {
		return
	}

	w.settings.vtbl.PutIsStatusBarEnabled.Call(uintptr(unsafe.Pointer(w.settings)), b2u(v))
}

func (w *WebView) SetErrorPageEnabled(v bool) {
	if w.settings == nil {
		return
	}

	w.settings.vtbl.PutIsBuiltInErrorPageEnabled.Call(uintptr(unsafe.Pointer(w.settings)), b2u(v))
}

func (w *WebView) SetZoomControlEnabled(v bool) {
	if w.settings == nil {
		return
	}

	w.settings.vtbl.PutIsZoomControlEnabled.Call(uintptr(unsafe.Pointer(w.settings)), b2u(v))
}

func (w *WebView) SetUserAgent(ua string) {
	if w.set6 == nil {
		return
	}

	u := utf16Ptr(ua)
	w.set6.vtbl.PutUserAgent.Call(uintptr(unsafe.Pointer(w.set6)), uintptr(unsafe.Pointer(u)))
}

func (w *WebView) SetBrowserAcceleratorKeys(enabled bool) {
	if w.set6 == nil {
		return
	}

	w.set6.vtbl.PutAreBrowserAcceleratorKeysEnabled.Call(uintptr(unsafe.Pointer(w.set6)), b2u(enabled))
}

func (w *WebView) SetAutofill(password, general bool) {
	if w.set6 == nil {
		return
	}

	w.set6.vtbl.PutIsPasswordAutosaveEnabled.Call(uintptr(unsafe.Pointer(w.set6)), b2u(password))
	w.set6.vtbl.PutIsGeneralAutofillEnabled.Call(uintptr(unsafe.Pointer(w.set6)), b2u(general))
}

func (w *WebView) SetPinchZoom(v bool) {
	if w.set6 == nil {
		return
	}

	w.set6.vtbl.PutIsPinchZoomEnabled.Call(uintptr(unsafe.Pointer(w.set6)), b2u(v))
}

func (w *WebView) SetSwipeNavigation(v bool) {
	if w.set6 == nil {
		return
	}

	w.set6.vtbl.PutIsSwipeNavigationEnabled.Call(uintptr(unsafe.Pointer(w.set6)), b2u(v))
}

func (w *WebView) SetPreferredColorScheme(scheme int) {
	if w.profile == nil {
		return
	}

	w.profile.vtbl.PutPreferredColorScheme.Call(uintptr(unsafe.Pointer(w.profile)), uintptr(scheme))
}

func (w *WebView) AddBrowserExtension(folder string,
	retorno func(ext *ICoreWebView2BrowserExtension, id, name string, err error),
) {
	if w.profile == nil {
		retorno(nil, "", "", erroTexto("interface de perfil com extensões indisponível neste runtime"))

		return
	}

	tratador := newAddExtCompletedHandler(func(hr uintptr, ext *ICoreWebView2BrowserExtension) uintptr {
		if Failed(hr) {
			retorno(nil, "", "", hrErr(hr, "AddBrowserExtension"))

			return SOK
		}

		var idp, namep *uint16
		ext.vtbl.GetID.Call(uintptr(unsafe.Pointer(ext)), uintptr(unsafe.Pointer(&idp)))
		ext.vtbl.GetName.Call(uintptr(unsafe.Pointer(ext)), uintptr(unsafe.Pointer(&namep)))

		ext.vtbl.AddRef.Call(uintptr(unsafe.Pointer(ext)))
		retorno(ext, takeString(idp), takeString(namep), nil)

		return SOK
	})
	pin(tratador)

	f := utf16Ptr(folder)
	hr, _, _ := w.profile.vtbl.AddBrowserExtension.Call(
		uintptr(unsafe.Pointer(w.profile)),
		uintptr(unsafe.Pointer(f)),
		uintptr(unsafe.Pointer(tratador)),
	)
	tratador.vtbl.Release.Call(uintptr(unsafe.Pointer(tratador)))

	if Failed(hr) {
		retorno(nil, "", "", hrErr(hr, "AddBrowserExtension"))
	}
}

func (w *WebView) GetBrowserExtensions(retorno func(list *ICoreWebView2BrowserExtensionList, err error)) {
	if w == nil || w.profile == nil {
		retorno(nil, erroTexto("interface de perfil com extensões indisponível neste runtime"))

		return
	}

	tratador := newGetExtCompletedHandler(func(hr uintptr, list *ICoreWebView2BrowserExtensionList) uintptr {
		if Failed(hr) {
			retorno(nil, hrErr(hr, "GetBrowserExtensions"))

			return SOK
		}

		if list != nil {
			list.vtbl.AddRef.Call(uintptr(unsafe.Pointer(list)))
		}

		retorno(list, nil)

		return SOK
	})
	pin(tratador)
	hr, _, _ := w.profile.vtbl.GetBrowserExtensions.Call(
		uintptr(unsafe.Pointer(w.profile)),
		uintptr(unsafe.Pointer(tratador)),
	)
	tratador.vtbl.Release.Call(uintptr(unsafe.Pointer(tratador)))

	if Failed(hr) {
		retorno(nil, hrErr(hr, "GetBrowserExtensions"))
	}
}

func (w *WebView) Register() {
	tok := &eventToken{}

	w.registrarNavegacaoBasica(tok)
	w.registrarMultimidia(tok)
	w.registrarEsquemaOverdrive(tok)

	w.registrarMenuContexto(tok)
	w.registrarEventosFinais(tok)
}

func (w *WebView) responderRequisicao(args *ICoreWebView2WebResourceRequestedEventArgs, body []byte,
	status int, reason, headers string,
) {
	if w == nil {
		return
	}

	env := w.ownerEnv
	if env == nil && theApp != nil {
		env = theApp.env
	}

	if env == nil || env.ptr == nil {
		return
	}

	stream := fluxoRespostaRequisicao(body)

	response, pronto := criarRespostaRequisicao(env, stream, status, reason, headers)
	if stream != 0 {
		iu := (*IUnknown)(unsafe.Pointer(stream))
		iu.vtbl.Release.Call(uintptr(unsafe.Pointer(iu)))
	}

	if !pronto {
		return
	}

	args.vtbl.PutResponse.Call(uintptr(unsafe.Pointer(args)), uintptr(unsafe.Pointer(response)))
	response.vtbl.Release.Call(uintptr(unsafe.Pointer(response)))
}

func fluxoRespostaRequisicao(body []byte) uintptr {
	if len(body) == 0 {
		return 0
	}

	stream, _, _ := pSHCreateMemStream.Call(uintptr(unsafe.Pointer(&body[0])), uintptr(len(body)))

	return stream
}

func criarRespostaRequisicao(env *Environment, stream uintptr, status int, reason, headers string) (
	*ICoreWebView2WebResourceResponse, bool,
) {
	var response *ICoreWebView2WebResourceResponse

	ptrMotivo := utf16Ptr(reason)
	ptrCabecalhos := utf16Ptr(headers)

	hr, _, _ := env.ptr.vtbl.CreateWebResourceResponse.Call(
		uintptr(unsafe.Pointer(env.ptr)),
		stream,
		uintptr(status),
		uintptr(unsafe.Pointer(ptrMotivo)),
		uintptr(unsafe.Pointer(ptrCabecalhos)),
		uintptr(unsafe.Pointer(&response)))

	if Failed(hr) || response == nil {
		return nil, false
	}

	return response, true
}

func (w *WebView) setMemoryLow(low bool) {
	if w.view19 == nil || w.memoryLow == low {
		return
	}

	v := uintptr(0)
	if low {
		v = 1
	}

	w.view19.vtbl.PutMemoryUsageTargetLevel.Call(uintptr(unsafe.Pointer(w.view19)), v)
	w.memoryLow = low
}

func (w *WebView) resolverControladores() {
	if p := queryInterface(unsafe.Pointer(w.ctrl), IIDICoreWebView2Controller2); p != nil {
		w.ctrl2 = (*ICoreWebView2Controller2)(p)
	}

	if p := queryInterface(unsafe.Pointer(w.ctrl), IIDICoreWebView2Controller3); p != nil {
		w.ctrl3 = (*ICoreWebView2Controller3)(p)
	}

	if p := queryInterface(unsafe.Pointer(w.ctrl), IIDICoreWebView2Controller4); p != nil {
		w.ctrl4 = (*ICoreWebView2Controller4)(p)
	}
}

func (w *WebView) resolverVisoes() {
	if p := queryInterface(unsafe.Pointer(w.view), IIDICoreWebView2_3); p != nil {
		w.view3 = (*ICoreWebView2_3)(p)
	}

	if p := queryInterface(unsafe.Pointer(w.view), IIDICoreWebView2_4); p != nil {
		w.view4 = (*ICoreWebView2_4)(p)
	}

	if p := queryInterface(unsafe.Pointer(w.view), IIDICoreWebView2_8); p != nil {
		w.view8 = (*ICoreWebView2_8)(p)
	}

	if p := queryInterface(unsafe.Pointer(w.view), IIDICoreWebView2_11); p != nil {
		w.view11 = (*ICoreWebView2_11)(p)
	}

	if p := queryInterface(unsafe.Pointer(w.view), IIDICoreWebView2_13); p != nil {
		w.view13 = (*ICoreWebView2_13)(p)
	}

	if p := queryInterface(unsafe.Pointer(w.view), IIDICoreWebView2_19); p != nil {
		w.view19 = (*ICoreWebView2_19)(p)
	}

	if p := queryInterface(unsafe.Pointer(w.view), IIDICoreWebView2_25); p != nil {
		w.view25 = (*ICoreWebView2_25)(p)
	}

	if p := queryInterface(unsafe.Pointer(w.view), IIDICoreWebView2_28); p != nil {
		w.view28 = (*ICoreWebView2_28)(p)
	}
}

func (w *WebView) resolverAjustes() {
	var s *ICoreWebView2Settings
	w.view.vtbl.GetSettings.Call(uintptr(unsafe.Pointer(w.view)), uintptr(unsafe.Pointer(&s)))

	w.settings = s
	if s != nil {
		if p := queryInterface(unsafe.Pointer(s), IIDICoreWebView2Settings6); p != nil {
			w.set6 = (*ICoreWebView2Settings6)(p)
		}
	}
}

func (w *WebView) resolverPerfil() {
	if w.view13 == nil {
		return
	}

	var perfil *ICoreWebView2Profile
	w.view13.vtbl.GetProfile.Call(uintptr(unsafe.Pointer(w.view13)), uintptr(unsafe.Pointer(&perfil)))

	if perfil == nil {
		return
	}

	if p := queryInterface(unsafe.Pointer(perfil), IIDICoreWebView2Profile7); p != nil {
		w.profile = (*ICoreWebView2Profile7)(p)
	}

	perfil.vtbl.Release.Call(uintptr(unsafe.Pointer(perfil)))
}

func (w *WebView) resumirDiagnostico() {
	w.Diag = fmt.Sprintf("ctrl2=%v ctrl3=%v ctrl4=%v view3=%v view13=%v settings=%v set6=%v profile7=%v",
		w.ctrl2 != nil, w.ctrl3 != nil, w.ctrl4 != nil, w.view3 != nil, w.view13 != nil,
		w.settings != nil, w.set6 != nil, w.profile != nil)
}

func (w *WebView) resolve() {
	if w == nil || w.ctrl == nil || w.view == nil {
		return
	}

	w.resolverControladores()
	w.resolverVisoes()
	w.resolverAjustes()
	w.resolverPerfil()
	w.resumirDiagnostico()
}

func (w *WebView) registrarOrigem(tok *eventToken) {
	sch := newSrcChangedHandler(func(s *ICoreWebView2) uintptr {
		if w.OnSourceChanged != nil {
			var up *uint16
			s.vtbl.GetSource.Call(uintptr(unsafe.Pointer(s)), uintptr(unsafe.Pointer(&up)))
			w.OnSourceChanged(takeString(up))
		}

		return SOK
	})
	pin(sch)
	w.view.vtbl.AddSourceChanged.Call(uintptr(unsafe.Pointer(w.view)), uintptr(unsafe.Pointer(sch)),
		uintptr(unsafe.Pointer(tok)))
	sch.vtbl.Release.Call(uintptr(unsafe.Pointer(sch)))
}

func (w *WebView) registrarInicioNavegacao(tok *eventToken) {
	sth := newNavStartingHandler(func(_ *ICoreWebView2, args *ICoreWebView2NavigationStartingEventArgs) uintptr {
		if w.OnNavigationStarting != nil {
			var up *uint16
			args.vtbl.GetURI.Call(uintptr(unsafe.Pointer(args)), uintptr(unsafe.Pointer(&up)))
			w.OnNavigationStarting(takeString(up))
		}

		return SOK
	})
	pin(sth)
	w.view.vtbl.AddNavigationStarting.Call(uintptr(unsafe.Pointer(w.view)), uintptr(unsafe.Pointer(sth)),
		uintptr(unsafe.Pointer(tok)))
	sth.vtbl.Release.Call(uintptr(unsafe.Pointer(sth)))
}

func (w *WebView) registrarNavegacaoCompleta(tok *eventToken) {
	navegacao := newNavCompletedHandler(func(_ *ICoreWebView2, args *ICoreWebView2NavigationCompletedEventArgs) uintptr {
		var ok int32
		args.vtbl.GetIsSuccess.Call(uintptr(unsafe.Pointer(args)), uintptr(unsafe.Pointer(&ok)))

		if w.OnNavigated != nil {
			w.OnNavigated(ok != 0)
		}

		return SOK
	})
	pin(navegacao)
	w.view.vtbl.AddNavigationCompleted.Call(uintptr(unsafe.Pointer(w.view)), uintptr(unsafe.Pointer(navegacao)),
		uintptr(unsafe.Pointer(tok)))
	navegacao.vtbl.Release.Call(uintptr(unsafe.Pointer(navegacao)))
}

func (w *WebView) registrarTitulo(tok *eventToken) {
	titulo := newTitleChangedHandler(func(s *ICoreWebView2) uintptr {
		if w.OnTitleChanged != nil {
			var tp *uint16
			s.vtbl.GetDocumentTitle.Call(uintptr(unsafe.Pointer(s)), uintptr(unsafe.Pointer(&tp)))
			w.OnTitleChanged(takeString(tp))
		}

		return SOK
	})
	pin(titulo)
	w.view.vtbl.AddDocumentTitleChanged.Call(uintptr(unsafe.Pointer(w.view)), uintptr(unsafe.Pointer(titulo)),
		uintptr(unsafe.Pointer(tok)))
	titulo.vtbl.Release.Call(uintptr(unsafe.Pointer(titulo)))
}

func (w *WebView) registrarNovaJanela(tok *eventToken) {
	novaJanela := newNewWindowHandler(func(_ *ICoreWebView2, args *ICoreWebView2NewWindowRequestedEventArgs) uintptr {
		var up *uint16
		args.vtbl.GetURI.Call(uintptr(unsafe.Pointer(args)), uintptr(unsafe.Pointer(&up)))
		uri := takeString(up)

		var iniciada int32

		hr, _, _ := args.vtbl.GetIsUserInitiated.Call(uintptr(unsafe.Pointer(args)), uintptr(unsafe.Pointer(&iniciada)))
		if Failed(hr) {
			iniciada = 1
		}

		handled := false
		if w.OnNewWindow != nil {
			handled = w.OnNewWindow(uri, iniciada != 0)
		}

		if handled {
			args.vtbl.PutHandled.Call(uintptr(unsafe.Pointer(args)), 1)
		}

		return SOK
	})
	pin(novaJanela)
	w.view.vtbl.AddNewWindowRequested.Call(uintptr(unsafe.Pointer(w.view)), uintptr(unsafe.Pointer(novaJanela)),
		uintptr(unsafe.Pointer(tok)))
	novaJanela.vtbl.Release.Call(uintptr(unsafe.Pointer(novaJanela)))
}

func (w *WebView) registrarPermissao(tok *eventToken) {
	permissao := newPermissionRequestedHandler(func(
		_ *ICoreWebView2, args *ICoreWebView2PermissionRequestedEventArgs,
	) uintptr {
		var kind uint32
		args.vtbl.GetPermissionKind.Call(uintptr(unsafe.Pointer(args)), uintptr(unsafe.Pointer(&kind)))

		var up *uint16
		args.vtbl.GetURI.Call(uintptr(unsafe.Pointer(args)), uintptr(unsafe.Pointer(&up)))
		uri := takeString(up)

		permitir := w.AllowPermissions
		if w.OnPermissionRequested != nil {
			permitir = w.OnPermissionRequested(uri, kind)
		}

		const estadoNegadoPermissaoWV2 = 2

		estado := uintptr(estadoNegadoPermissaoWV2)
		if permitir {
			estado = 1
		}

		args.vtbl.PutState.Call(uintptr(unsafe.Pointer(args)), estado)

		return SOK
	})
	pin(permissao)
	w.view.vtbl.AddPermissionRequested.Call(uintptr(unsafe.Pointer(w.view)), uintptr(unsafe.Pointer(permissao)),
		uintptr(unsafe.Pointer(tok)))
	permissao.vtbl.Release.Call(uintptr(unsafe.Pointer(permissao)))
}

func (w *WebView) registrarNavegacaoBasica(tok *eventToken) {
	w.registrarOrigem(tok)
	w.registrarInicioNavegacao(tok)
	w.registrarNavegacaoCompleta(tok)
	w.registrarTitulo(tok)
	w.registrarNovaJanela(tok)
	w.registrarPermissao(tok)
}

func (w *WebView) registrarDownload(tok *eventToken) {
	if w.view4 == nil {
		return
	}

	descarga := newDownloadStartingHandler(func(_ *ICoreWebView2, args *ICoreWebView2DownloadStartingEventArgs) uintptr {
		if w.OnDownloadStarting != nil {
			w.OnDownloadStarting(args)
		}

		return SOK
	})
	pin(descarga)
	w.view4.vtbl.AddDownloadStarting.Call(uintptr(unsafe.Pointer(w.view4)), uintptr(unsafe.Pointer(descarga)),
		uintptr(unsafe.Pointer(tok)))
	descarga.vtbl.Release.Call(uintptr(unsafe.Pointer(descarga)))
}

func (w *WebView) registrarAudio(tok *eventToken) {
	if w.view8 == nil {
		return
	}

	mudo := newIsMutedChangedHandler(func(_ *ICoreWebView2, _ *IUnknown) uintptr {
		var m int32
		w.view8.vtbl.GetIsMuted.Call(uintptr(unsafe.Pointer(w.view8)), uintptr(unsafe.Pointer(&m)))

		if w.OnMutedChanged != nil {
			w.OnMutedChanged(m != 0)
		}

		return SOK
	})
	pin(mudo)
	w.view8.vtbl.AddIsMutedChanged.Call(uintptr(unsafe.Pointer(w.view8)), uintptr(unsafe.Pointer(mudo)),
		uintptr(unsafe.Pointer(tok)))
	mudo.vtbl.Release.Call(uintptr(unsafe.Pointer(mudo)))

	audio := newIsDocumentPlayingAudioChangedHandler(func(_ *ICoreWebView2, _ *IUnknown) uintptr {
		var p int32
		w.view8.vtbl.GetIsDocumentPlayingAudio.Call(uintptr(unsafe.Pointer(w.view8)), uintptr(unsafe.Pointer(&p)))

		if w.OnAudioChanged != nil {
			w.OnAudioChanged(p != 0)
		}

		return SOK
	})
	pin(audio)
	w.view8.vtbl.AddIsDocumentPlayingAudioChanged.Call(uintptr(unsafe.Pointer(w.view8)), uintptr(unsafe.Pointer(audio)),
		uintptr(unsafe.Pointer(tok)))
	audio.vtbl.Release.Call(uintptr(unsafe.Pointer(audio)))
}

func (w *WebView) registrarMultimidia(tok *eventToken) {
	w.registrarDownload(tok)
	w.registrarAudio(tok)
}

func (w *WebView) responderEsquemaOverdrive(args *ICoreWebView2WebResourceRequestedEventArgs) uintptr {
	if _, ok := contextoRequisicaoOD(args); !ok {
		return SOK
	}

	uri, ok := extrairURIRequisicao(args)
	if !ok {
		return SOK
	}

	if !strings.HasPrefix(strings.ToLower(uri), "overdrive://") {
		return SOK
	}

	body, status, reason, headers := responderPaginaInterna(uri)
	w.responderRequisicao(args, body, status, reason, headers)

	return SOK
}

func (w *WebView) registrarEsquemaOverdrive(tok *eventToken) {
	instalarPreFiltro(w)

	filtroOD := utf16Ptr("overdrive://*")
	w.view.vtbl.AddWebResourceRequestedFilter.Call(
		uintptr(unsafe.Pointer(w.view)),
		uintptr(unsafe.Pointer(filtroOD)), 0)

	requisicao := newWebResourceRequestedHandler(func(
		_ *ICoreWebView2, args *ICoreWebView2WebResourceRequestedEventArgs,
	) uintptr {
		return w.responderEsquemaOverdrive(args)
	})
	pin(requisicao)
	w.view.vtbl.AddWebResourceRequested.Call(
		uintptr(unsafe.Pointer(w.view)),
		uintptr(unsafe.Pointer(requisicao)),
		uintptr(unsafe.Pointer(tok)))
	requisicao.vtbl.Release.Call(uintptr(unsafe.Pointer(requisicao)))
}

func (w *WebView) adicionarItemMenuContexto(
	env9 *ICoreWebView2Environment9, itens *ICoreWebView2ContextMenuItemCollection,
	label, id, valor string, posicao int,
) {
	var item *ICoreWebView2ContextMenuItem

	hr, _, _ := env9.vtbl.CreateContextMenuItem.Call(
		uintptr(unsafe.Pointer(env9)),
		uintptr(unsafe.Pointer(utf16Ptr(label))), 0, 0,
		uintptr(unsafe.Pointer(&item)))
	if Failed(hr) || item == nil {
		return
	}

	var n uint32
	itens.vtbl.GetCount.Call(uintptr(unsafe.Pointer(itens)), uintptr(unsafe.Pointer(&n)))

	pos := uintptr(n)
	if posicao >= 0 && uint32(posicao) < n {
		pos = uintptr(uint32(posicao))
	}

	itens.vtbl.InsertValueAtIndex.Call(uintptr(unsafe.Pointer(itens)), pos, uintptr(unsafe.Pointer(item)))

	selecao := newCustomItemSelectedHandler(func(_ *ICoreWebView2ContextMenuItem, _ *IUnknown) uintptr {
		if w.OnContextAction != nil {
			w.OnContextAction(id, valor)
		}

		return SOK
	})

	var itemTok eventToken
	item.vtbl.AddCustomItemSelected.Call(
		uintptr(unsafe.Pointer(item)),
		uintptr(unsafe.Pointer(selecao)),
		uintptr(unsafe.Pointer(&itemTok)))

	if app2 := theApp; app2 != nil {
		app2.ctxHandlers = append(app2.ctxHandlers, pin(selecao))
	}

	selecao.vtbl.Release.Call(uintptr(unsafe.Pointer(selecao)))
	item.vtbl.Release.Call(uintptr(unsafe.Pointer(item)))
}

func (w *WebView) inserirItensLinkContexto(add adicionadorItemContexto, link string) {
	if link == "" {
		return
	}

	add(txtCtxAbrirLinkNova, "link:newtab", link)
	add(txtCtxCopiarLink, "link:copy", link)
}

func (w *WebView) inserirItensMidiaContexto(env9 *ICoreWebView2Environment9,
	itens *ICoreWebView2ContextMenuItemCollection, fonte string,
) {
	if fonte == "" {
		return
	}

	addEm := func(label, id, valor string, em int) {
		w.adicionarItemMenuContexto(env9, itens, label, id, valor, em)
	}
	add := func(label, id, valor string) {
		addEm(label, id, valor, -1)
	}

	indiceSalvar := indiceSalvarImagemContexto(itens)

	if indiceSalvar >= 0 {
		addEm(txtCtxAbrirImgNova, "img:newtab", fonte, indiceSalvar)
		addEm(txtCtxCopiarURLImg, "img:copy", fonte, indiceSalvar+1)
	} else {
		add(txtCtxAbrirImgNova, "img:newtab", fonte)
		add(txtCtxCopiarURLImg, "img:copy", fonte)
	}
}

func (w *WebView) inserirItensSelecaoContexto(add adicionadorItemContexto, selecao string) {
	if selecao == "" {
		return
	}

	add(txtCtxPesquisarSel, "sel:search", selecao)
	add(txtCtxCopiarSel, "sel:copy", selecao)
}

func (w *WebView) cuidarMenuContexto(menuEnv *Environment,
	args *ICoreWebView2ContextMenuRequestedEventArgs,
) uintptr {
	if app := theApp; app != nil {
		app.limparHandlersContexto()
	}

	var target *ICoreWebView2ContextMenuTarget

	hr, _, _ := args.vtbl.GetContextMenuTarget.Call(uintptr(unsafe.Pointer(args)), uintptr(unsafe.Pointer(&target)))
	if Failed(hr) || target == nil {
		return SOK
	}
	defer target.vtbl.Release.Call(uintptr(unsafe.Pointer(target)))

	link, fonte, selecao := extrairTextosAlvoContexto(target)

	if link == "" && fonte == "" && selecao == "" {
		return SOK
	}

	env9, pronto := abrirAmbienteContexto9(menuEnv)
	if !pronto {
		return SOK
	}
	defer env9.vtbl.Release.Call(uintptr(unsafe.Pointer(env9)))

	itens, pronto := coletarItensMenuContexto(args)
	if !pronto {
		return SOK
	}
	defer itens.vtbl.Release.Call(uintptr(unsafe.Pointer(itens)))

	addEm := func(label, id, valor string, em int) {
		w.adicionarItemMenuContexto(env9, itens, label, id, valor, em)
	}
	add := func(label, id, valor string) {
		addEm(label, id, valor, -1)
	}

	w.inserirItensLinkContexto(add, link)
	w.inserirItensMidiaContexto(env9, itens, fonte)
	w.inserirItensSelecaoContexto(add, selecao)

	return SOK
}

func (w *WebView) registrarMenuContexto(tok *eventToken) {
	menuEnv := ambienteMenuContexto(w)

	if w.view11 == nil || menuEnv == nil || w.OnContextAction == nil {
		return
	}

	menu := newContextMenuRequestedHandler(func(_ *ICoreWebView2,
		args *ICoreWebView2ContextMenuRequestedEventArgs,
	) uintptr {
		return w.cuidarMenuContexto(menuEnv, args)
	})
	pin(menu)
	w.view11.vtbl.AddContextMenuRequested.Call(
		uintptr(unsafe.Pointer(w.view11)),
		uintptr(unsafe.Pointer(menu)),
		uintptr(unsafe.Pointer(tok)))
	menu.vtbl.Release.Call(uintptr(unsafe.Pointer(menu)))
}

func (w *WebView) registrarFalhaProcesso(tok *eventToken) {
	falha := newProcessFailedHandler(func(_ *ICoreWebView2, args *ICoreWebView2ProcessFailedEventArgs) uintptr {
		if w.OnProcessFail != nil {
			var kind uint32
			args.vtbl.GetProcessFailedKind.Call(uintptr(unsafe.Pointer(args)), uintptr(unsafe.Pointer(&kind)))
			w.OnProcessFail(kind)
		}

		return SOK
	})
	pin(falha)
	w.view.vtbl.AddProcessFailed.Call(uintptr(unsafe.Pointer(w.view)), uintptr(unsafe.Pointer(falha)),
		uintptr(unsafe.Pointer(tok)))
	falha.vtbl.Release.Call(uintptr(unsafe.Pointer(falha)))
}

func (w *WebView) cuidarTeclaAtalho(args *ICoreWebView2AcceleratorKeyPressedEventArgs) uintptr {
	if w.OnAccelKey == nil {
		return SOK
	}

	var (
		kind        uint32
		tecla       uint32
		teclaFisica physicalKeyStatus
	)

	args.vtbl.GetKeyEventKind.Call(uintptr(unsafe.Pointer(args)), uintptr(unsafe.Pointer(&kind)))
	args.vtbl.GetVirtualKey.Call(uintptr(unsafe.Pointer(args)), uintptr(unsafe.Pointer(&tecla)))
	args.vtbl.GetPhysicalKeyStatus.Call(uintptr(unsafe.Pointer(args)), uintptr(unsafe.Pointer(&teclaFisica)))

	if teclaFisica.WasKeyDown != 0 {
		return SOK
	}

	const (
		teclaControleWV2 = 0x11
		teclaShiftWV2    = 0x10
		teclaAltWV2      = 0x12
		tipoTeclaSolta   = 2
	)

	down := kind == 0 || kind == tipoTeclaSolta
	ctrl := teclaPressionada(teclaControleWV2)
	shift := teclaPressionada(teclaShiftWV2)

	alt := teclaPressionada(teclaAltWV2) || kind == tipoTeclaSolta

	if !down {
		return SOK
	}

	if w.OnAccelKey(tecla, ctrl, shift, alt) {
		args.vtbl.PutHandled.Call(uintptr(unsafe.Pointer(args)), 1)
	}

	return SOK
}

func (w *WebView) registrarTeclaAtalho(tok *eventToken) {
	atalho := newAccelKeyHandler(func(
		_ *ICoreWebView2Controller, args *ICoreWebView2AcceleratorKeyPressedEventArgs,
	) uintptr {
		return w.cuidarTeclaAtalho(args)
	})
	pin(atalho)
	w.ctrl.vtbl.AddAcceleratorKeyPressed.Call(uintptr(unsafe.Pointer(w.ctrl)), uintptr(unsafe.Pointer(atalho)),
		uintptr(unsafe.Pointer(tok)))
	atalho.vtbl.Release.Call(uintptr(unsafe.Pointer(atalho)))
}

func (w *WebView) registrarFocoMensagem(tok *eventToken) {
	foco := newGotFocusHandler(func() uintptr {
		if w.OnGotFocus != nil {
			w.OnGotFocus()
		}

		return SOK
	})
	pin(foco)
	w.ctrl.vtbl.AddGotFocus.Call(uintptr(unsafe.Pointer(w.ctrl)), uintptr(unsafe.Pointer(foco)),
		uintptr(unsafe.Pointer(tok)))
	foco.vtbl.Release.Call(uintptr(unsafe.Pointer(foco)))

	mensagem := newWebMessageHandler(func(_ *ICoreWebView2, args *ICoreWebView2WebMessageReceivedEventArgs) uintptr {
		if w.OnMessage == nil {
			return SOK
		}

		var mp *uint16
		args.vtbl.TryGetWebMessageAsString.Call(uintptr(unsafe.Pointer(args)), uintptr(unsafe.Pointer(&mp)))

		if mp != nil {
			w.OnMessage(takeString(mp))
		}

		return SOK
	})
	pin(mensagem)
	w.view.vtbl.AddWebMessageReceived.Call(uintptr(unsafe.Pointer(w.view)), uintptr(unsafe.Pointer(mensagem)),
		uintptr(unsafe.Pointer(tok)))
	mensagem.vtbl.Release.Call(uintptr(unsafe.Pointer(mensagem)))
}

func (w *WebView) registrarEventosFinais(tok *eventToken) {
	w.registrarFalhaProcesso(tok)
	w.registrarTeclaAtalho(tok)
	w.registrarFocoMensagem(tok)
}

func (w *WebView) liberarVisoes() {
	if w.view19 != nil {
		w.view19.vtbl.Release.Call(uintptr(unsafe.Pointer(w.view19)))
		w.view19 = nil
	}

	if w.view25 != nil {
		w.view25.vtbl.Release.Call(uintptr(unsafe.Pointer(w.view25)))
		w.view25 = nil
	}

	if w.view28 != nil {
		w.view28.vtbl.Release.Call(uintptr(unsafe.Pointer(w.view28)))
		w.view28 = nil
	}

	if w.view13 != nil {
		w.view13.vtbl.Release.Call(uintptr(unsafe.Pointer(w.view13)))
		w.view13 = nil
	}

	if w.view11 != nil {
		w.view11.vtbl.Release.Call(uintptr(unsafe.Pointer(w.view11)))
		w.view11 = nil
	}

	if w.view8 != nil {
		w.view8.vtbl.Release.Call(uintptr(unsafe.Pointer(w.view8)))
		w.view8 = nil
	}

	if w.view4 != nil {
		w.view4.vtbl.Release.Call(uintptr(unsafe.Pointer(w.view4)))
		w.view4 = nil
	}

	if w.view3 != nil {
		w.view3.vtbl.Release.Call(uintptr(unsafe.Pointer(w.view3)))
		w.view3 = nil
	}
}

func (w *WebView) liberarControladoresEAjustes() {
	if w.ctrl4 != nil {
		w.ctrl4.vtbl.Release.Call(uintptr(unsafe.Pointer(w.ctrl4)))
		w.ctrl4 = nil
	}

	if w.ctrl3 != nil {
		w.ctrl3.vtbl.Release.Call(uintptr(unsafe.Pointer(w.ctrl3)))
		w.ctrl3 = nil
	}

	if w.ctrl2 != nil {
		w.ctrl2.vtbl.Release.Call(uintptr(unsafe.Pointer(w.ctrl2)))
		w.ctrl2 = nil
	}

	if w.set6 != nil {
		w.set6.vtbl.Release.Call(uintptr(unsafe.Pointer(w.set6)))
		w.set6 = nil
	}

	if w.profile != nil {
		w.profile.vtbl.Release.Call(uintptr(unsafe.Pointer(w.profile)))
		w.profile = nil
	}

	if w.settings != nil {
		w.settings.vtbl.Release.Call(uintptr(unsafe.Pointer(w.settings)))
		w.settings = nil
	}
}

func (w *WebView) liberarNucleo() {
	if w.view != nil {
		w.view.vtbl.Release.Call(uintptr(unsafe.Pointer(w.view)))
		w.view = nil
	}

	if w.ctrl != nil {
		w.ctrl.vtbl.Release.Call(uintptr(unsafe.Pointer(w.ctrl)))
		w.ctrl = nil
	}
}

func (w *WebView) limparCallbacks() {
	w.OnSourceChanged = nil
	w.OnNavigationStarting = nil
	w.OnNavigated = nil
	w.OnTitleChanged = nil
	w.OnNewWindow = nil
	w.OnPermissionRequested = nil
	w.OnDownloadStarting = nil
	w.OnMutedChanged = nil
	w.OnAudioChanged = nil
	w.OnContextAction = nil
	w.OnProcessFail = nil
	w.OnAccelKey = nil
	w.OnGotFocus = nil
	w.OnMessage = nil
	w.AllowPermissions = false
	w.ownerEnv = nil
	w.extIDs = nil
	w.extPopups = nil
}

func (e *ICoreWebView2BrowserExtension) Remove(retorno func(error)) {
	if e == nil || e.vtbl == nil {
		if retorno != nil {
			retorno(erroTexto("extensão inválida"))
		}

		return
	}

	tratador := newRemoveExtCompletedHandler(func(hr uintptr) uintptr {
		if retorno != nil {
			if Failed(hr) {
				retorno(hrErr(hr, "RemoveBrowserExtension"))
			} else {
				retorno(nil)
			}
		}

		return SOK
	})
	pin(tratador)
	hr, _, _ := e.vtbl.Remove.Call(uintptr(unsafe.Pointer(e)), uintptr(unsafe.Pointer(tratador)))
	tratador.vtbl.Release.Call(uintptr(unsafe.Pointer(tratador)))

	if Failed(hr) && retorno != nil {
		retorno(hrErr(hr, "RemoveBrowserExtension"))
	}
}

func (e *ICoreWebView2BrowserExtension) Release() {
	if e == nil || e.vtbl == nil {
		return
	}

	e.vtbl.Release.Call(uintptr(unsafe.Pointer(e)))
}

func (e *ICoreWebView2BrowserExtension) Enable(ligar bool, retorno func(error)) {
	if e == nil || e.vtbl == nil {
		if retorno != nil {
			retorno(erroTexto("extensão inválida"))
		}

		return
	}

	tratador := newEnableExtCompletedHandler(func(hr uintptr) uintptr {
		if retorno != nil {
			if Failed(hr) {
				retorno(hrErr(hr, "EnableBrowserExtension"))
			} else {
				retorno(nil)
			}
		}

		return SOK
	})
	pin(tratador)

	var v uintptr
	if ligar {
		v = 1
	}

	hr, _, _ := e.vtbl.Enable.Call(uintptr(unsafe.Pointer(e)), v, uintptr(unsafe.Pointer(tratador)))
	tratador.vtbl.Release.Call(uintptr(unsafe.Pointer(tratador)))

	if Failed(hr) && retorno != nil {
		retorno(hrErr(hr, "EnableBrowserExtension"))
	}
}

func (e *ICoreWebView2BrowserExtension) IsEnabled() (bool, error) {
	if e == nil || e.vtbl == nil {
		return false, erroTexto("extensão inválida")
	}

	var valor int32

	hr, _, _ := e.vtbl.GetIsEnabled.Call(uintptr(unsafe.Pointer(e)), uintptr(unsafe.Pointer(&valor)))
	if Failed(hr) {
		return false, hrErr(hr, "GetIsEnabled")
	}

	return valor != 0, nil
}

func (l *ICoreWebView2BrowserExtensionList) Release() {
	if l == nil || l.vtbl == nil {
		return
	}

	l.vtbl.Release.Call(uintptr(unsafe.Pointer(l)))
}

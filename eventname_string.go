// Code generated by "enumer -type=EventName"; DO NOT EDIT

package goesl

import (
	"fmt"
)

const _EventName_name = "CUSTOMCLONECHANNEL_CREATECHANNEL_DESTROYCHANNEL_STATECHANNEL_CALLSTATECHANNEL_ANSWERCHANNEL_HANGUPCHANNEL_HANGUP_COMPLETECHANNEL_EXECUTECHANNEL_EXECUTE_COMPLETECHANNEL_HOLDCHANNEL_UNHOLDCHANNEL_BRIDGECHANNEL_UNBRIDGECHANNEL_PROGRESSCHANNEL_PROGRESS_MEDIACHANNEL_OUTGOINGCHANNEL_PARKCHANNEL_UNPARKCHANNEL_APPLICATIONCHANNEL_ORIGINATECHANNEL_UUIDAPILOGINBOUND_CHANOUTBOUND_CHANSTARTUPSHUTDOWNPUBLISHUNPUBLISHTALKNOTALKSESSION_CRASHMODULE_LOADMODULE_UNLOADDTMFMESSAGEPRESENCE_INNOTIFY_INPRESENCE_OUTPRESENCE_PROBEMESSAGE_WAITINGMESSAGE_QUERYROSTERCODECBACKGROUND_JOBDETECTED_SPEECHDETECTED_TONEPRIVATE_COMMANDHEARTBEATTRAPADD_SCHEDULEDEL_SCHEDULEEXE_SCHEDULERE_SCHEDULERELOADXMLNOTIFYPHONE_FEATUREPHONE_FEATURE_SUBSCRIBESEND_MESSAGERECV_MESSAGEREQUEST_PARAMSCHANNEL_DATAGENERALCOMMANDSESSION_HEARTBEATCLIENT_DISCONNECTEDSERVER_DISCONNECTEDSEND_INFORECV_INFORECV_RTCP_MESSAGECALL_SECURENATRECORD_STARTRECORD_STOPPLAYBACK_STARTPLAYBACK_STOPCALL_UPDATEFAILURESOCKET_DATAMEDIA_BUG_STARTMEDIA_BUG_STOPCONFERENCE_DATA_QUERYCONFERENCE_DATACALL_SETUP_REQCALL_SETUP_RESULTCALL_DETAILDEVICE_STATEALL"

var _EventName_index = [...]uint16{0, 6, 11, 25, 40, 53, 70, 84, 98, 121, 136, 160, 172, 186, 200, 216, 232, 254, 270, 282, 296, 315, 332, 344, 347, 350, 362, 375, 382, 390, 397, 406, 410, 416, 429, 440, 453, 457, 464, 475, 484, 496, 510, 525, 538, 544, 549, 563, 578, 591, 606, 615, 619, 631, 643, 655, 666, 675, 681, 694, 717, 729, 741, 755, 767, 774, 781, 798, 817, 836, 845, 854, 871, 882, 885, 897, 908, 922, 935, 946, 953, 964, 979, 993, 1014, 1029, 1043, 1060, 1071, 1083, 1086}

func (i EventName) String() string {
	if i < 0 || i >= EventName(len(_EventName_index)-1) {
		return fmt.Sprintf("EventName(%d)", i)
	}
	return _EventName_name[_EventName_index[i]:_EventName_index[i+1]]
}

var _EventNameNameToValue_map = map[string]EventName{
	_EventName_name[0:6]:       0,
	_EventName_name[6:11]:      1,
	_EventName_name[11:25]:     2,
	_EventName_name[25:40]:     3,
	_EventName_name[40:53]:     4,
	_EventName_name[53:70]:     5,
	_EventName_name[70:84]:     6,
	_EventName_name[84:98]:     7,
	_EventName_name[98:121]:    8,
	_EventName_name[121:136]:   9,
	_EventName_name[136:160]:   10,
	_EventName_name[160:172]:   11,
	_EventName_name[172:186]:   12,
	_EventName_name[186:200]:   13,
	_EventName_name[200:216]:   14,
	_EventName_name[216:232]:   15,
	_EventName_name[232:254]:   16,
	_EventName_name[254:270]:   17,
	_EventName_name[270:282]:   18,
	_EventName_name[282:296]:   19,
	_EventName_name[296:315]:   20,
	_EventName_name[315:332]:   21,
	_EventName_name[332:344]:   22,
	_EventName_name[344:347]:   23,
	_EventName_name[347:350]:   24,
	_EventName_name[350:362]:   25,
	_EventName_name[362:375]:   26,
	_EventName_name[375:382]:   27,
	_EventName_name[382:390]:   28,
	_EventName_name[390:397]:   29,
	_EventName_name[397:406]:   30,
	_EventName_name[406:410]:   31,
	_EventName_name[410:416]:   32,
	_EventName_name[416:429]:   33,
	_EventName_name[429:440]:   34,
	_EventName_name[440:453]:   35,
	_EventName_name[453:457]:   36,
	_EventName_name[457:464]:   37,
	_EventName_name[464:475]:   38,
	_EventName_name[475:484]:   39,
	_EventName_name[484:496]:   40,
	_EventName_name[496:510]:   41,
	_EventName_name[510:525]:   42,
	_EventName_name[525:538]:   43,
	_EventName_name[538:544]:   44,
	_EventName_name[544:549]:   45,
	_EventName_name[549:563]:   46,
	_EventName_name[563:578]:   47,
	_EventName_name[578:591]:   48,
	_EventName_name[591:606]:   49,
	_EventName_name[606:615]:   50,
	_EventName_name[615:619]:   51,
	_EventName_name[619:631]:   52,
	_EventName_name[631:643]:   53,
	_EventName_name[643:655]:   54,
	_EventName_name[655:666]:   55,
	_EventName_name[666:675]:   56,
	_EventName_name[675:681]:   57,
	_EventName_name[681:694]:   58,
	_EventName_name[694:717]:   59,
	_EventName_name[717:729]:   60,
	_EventName_name[729:741]:   61,
	_EventName_name[741:755]:   62,
	_EventName_name[755:767]:   63,
	_EventName_name[767:774]:   64,
	_EventName_name[774:781]:   65,
	_EventName_name[781:798]:   66,
	_EventName_name[798:817]:   67,
	_EventName_name[817:836]:   68,
	_EventName_name[836:845]:   69,
	_EventName_name[845:854]:   70,
	_EventName_name[854:871]:   71,
	_EventName_name[871:882]:   72,
	_EventName_name[882:885]:   73,
	_EventName_name[885:897]:   74,
	_EventName_name[897:908]:   75,
	_EventName_name[908:922]:   76,
	_EventName_name[922:935]:   77,
	_EventName_name[935:946]:   78,
	_EventName_name[946:953]:   79,
	_EventName_name[953:964]:   80,
	_EventName_name[964:979]:   81,
	_EventName_name[979:993]:   82,
	_EventName_name[993:1014]:  83,
	_EventName_name[1014:1029]: 84,
	_EventName_name[1029:1043]: 85,
	_EventName_name[1043:1060]: 86,
	_EventName_name[1060:1071]: 87,
	_EventName_name[1071:1083]: 88,
	_EventName_name[1083:1086]: 89,
}

func EventNameString(s string) (EventName, error) {
	if val, ok := _EventNameNameToValue_map[s]; ok {
		return val, nil
	}
	return 0, fmt.Errorf("%s does not belong to EventName values", s)
}
